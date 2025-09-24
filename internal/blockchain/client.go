package blockchain

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/darrenvechain/thorgo/crypto/tx"
	"github.com/darrenvechain/thorgo/thorest"
	"github.com/ethereum/go-ethereum/common"
)

type Client struct {
	thorClient *thorest.Client
	config     Config
	cache      *BalanceCache
	mu         sync.RWMutex
	status     NetworkStatus
}

const (
	DefaultMainnetURL = "https://mainnet.veblocks.net"
	DefaultTestnetURL = "https://testnet.veblocks.net"
	DefaultTimeout    = 30 * time.Second
	DefaultRetryCount = 3
	DefaultRetryDelay = 2 * time.Second
	DefaultCacheTTL   = 30 * time.Second
)

func NewClient(config Config) (*Client, error) {
	if config.NodeURL == "" {
		switch config.Network {
		case MainNet:
			config.NodeURL = DefaultMainnetURL
		case TestNet:
			config.NodeURL = DefaultTestnetURL
		default:
			return nil, fmt.Errorf("unknown network: %s", config.Network)
		}
	}

	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}
	if config.RetryCount == 0 {
		config.RetryCount = DefaultRetryCount
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = DefaultRetryDelay
	}

	thorClient := thorest.NewClientFromURL(config.NodeURL)

	c := &Client{
		thorClient: thorClient,
		config:     config,
		cache:      NewBalanceCache(DefaultCacheTTL),
		status: NetworkStatus{
			NodeURL:     config.NodeURL,
			Connected:   false,
			LastChecked: time.Now(),
		},
	}

	c.cache.StartCleanupRoutine(5 * time.Minute)

	if err := c.checkConnection(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Client) checkConnection() error {
	best, err := c.thorClient.BestBlock()
	if err != nil {
		c.updateStatus(false, 0, "")
		return NewNetworkError("failed to connect to VeChain network", err)
	}

	c.updateStatus(true, uint64(best.Number), best.ID.String())
	return nil
}

func (c *Client) updateStatus(connected bool, blockHeight uint64, networkID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.status.Connected = connected
	c.status.BlockHeight = blockHeight
	c.status.NetworkID = networkID
	c.status.LastChecked = time.Now()
}

func (c *Client) GetStatus() NetworkStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.status
}

// AddressExists checks if an address exists on the blockchain by attempting to get its account info
func (c *Client) AddressExists(address string) (bool, error) {
	addr := common.HexToAddress(address)
	_, err := c.thorClient.Account(addr)
	if err != nil {
		return false, NewNetworkError("failed to check address existence", err)
	}

	// If we can get the account info, the address exists on the blockchain
	// Note: On VeChain, accounts don't need to be "created" to exist, but this check
	// verifies we can query the address
	return true, nil
}

func (c *Client) GetBalance(address string) (*Balance, error) {
	if cached, found := c.cache.Get(address); found {
		return cached, nil
	}

	balance, err := c.fetchBalanceFromNetwork(address)
	if err != nil {
		return nil, err
	}

	c.cache.Set(address, balance)
	return balance, nil
}

func (c *Client) fetchBalanceFromNetwork(address string) (*Balance, error) {
	var lastErr error

	for attempt := 0; attempt < c.config.RetryCount; attempt++ {
		if attempt > 0 {
			time.Sleep(c.config.RetryDelay * time.Duration(attempt))
		}

		balance, err := c.doFetchBalance(address)
		if err == nil {
			return balance, nil
		}

		lastErr = err
		if blockchainErr := ClassifyError(err); blockchainErr != nil && !blockchainErr.IsRetryable() {
			break
		}
	}

	return nil, ClassifyError(lastErr)
}

func (c *Client) doFetchBalance(address string) (*Balance, error) {
	vetBalance, err := c.GetVETBalance(address)
	if err != nil {
		return nil, err
	}

	vthoBalance, err := c.GetVTHOBalance(address)
	if err != nil {
		return nil, err
	}

	return &Balance{
		VET:         vetBalance,
		VTHO:        vthoBalance,
		LastUpdated: time.Now(),
	}, nil
}

func (c *Client) GetVETBalance(address string) (*big.Int, error) {
	addr := common.HexToAddress(address)
	account, err := c.thorClient.Account(addr)
	if err != nil {
		return nil, NewNetworkError("failed to get VET balance", err)
	}

	return account.Balance.ToInt(), nil
}

func (c *Client) GetVTHOBalance(address string) (*big.Int, error) {
	addr := common.HexToAddress(address)
	account, err := c.thorClient.Account(addr)
	if err != nil {
		return nil, NewNetworkError("failed to get VTHO balance", err)
	}

	return account.Energy.ToInt(), nil
}

func (c *Client) RefreshBalance(address string) (*Balance, error) {
	c.cache.Invalidate(address)
	return c.GetBalance(address)
}

func (c *Client) GetCachedBalance(address string) (*Balance, bool) {
	return c.cache.Get(address)
}

func (c *Client) InvalidateCache(address string) {
	c.cache.Invalidate(address)
}

func (c *Client) EstimateGas(tx *Transaction) (*big.Int, error) {
	var gasLimit *big.Int

	switch tx.Asset {
	case VET:
		gasLimit = big.NewInt(21000)
	case VTHO:
		gasLimit = big.NewInt(80000)
	default:
		return nil, fmt.Errorf("unsupported asset type: %s", tx.Asset)
	}

	return gasLimit, nil
}

func (c *Client) BuildTransaction(from, to string, amount *big.Int, asset AssetType) (*Transaction, error) {
	gasLimit, err := c.EstimateGas(&Transaction{Asset: asset})
	if err != nil {
		return nil, err
	}

	// Get chain tag and best block for transaction construction
	chainTag, err := c.thorClient.ChainTag()
	if err != nil {
		return nil, NewNetworkError("failed to get chain tag", err)
	}

	bestBlock, err := c.thorClient.BestBlock()
	if err != nil {
		return nil, NewNetworkError("failed to get best block", err)
	}

	// Create block reference from best block
	blockRef := tx.NewBlockRef(uint32(bestBlock.Number))

	// Create transaction clause
	toAddr := common.HexToAddress(to)
	clause := tx.NewClause(&toAddr).WithValue(amount)

	// For VTHO transfers, we need to add contract call data
	if asset == VTHO {
		// VTHO is transferred via the energy contract
		// This is a simplified implementation - in practice you'd need the proper contract ABI
		clause = clause.WithData([]byte{}) // Placeholder for VTHO transfer data
	}

	// Build the transaction
	thorTx := tx.NewBuilder(tx.TypeLegacy).
		ChainTag(chainTag).
		BlockRef(blockRef).
		Expiration(32). // 32 blocks expiration
		Gas(gasLimit.Uint64()).
		GasPriceCoef(0).
		Clause(clause).
		Build()

	return &Transaction{
		From:     from,
		To:       to,
		Amount:   amount,
		Asset:    asset,
		GasLimit: gasLimit,
		GasPrice: big.NewInt(1000000000000000),
		Status:   StatusPending,
		TxID:     thorTx.ID().String(),
	}, nil
}

func (c *Client) SignTransaction(transaction *Transaction, privateKey *ecdsa.PrivateKey) (*tx.Transaction, error) {
	// Get chain tag and best block for transaction construction
	chainTag, err := c.thorClient.ChainTag()
	if err != nil {
		return nil, NewNetworkError("failed to get chain tag", err)
	}

	bestBlock, err := c.thorClient.BestBlock()
	if err != nil {
		return nil, NewNetworkError("failed to get best block", err)
	}

	// Create block reference from best block
	blockRef := tx.NewBlockRef(uint32(bestBlock.Number))

	// Create transaction clause
	toAddr := common.HexToAddress(transaction.To)
	clause := tx.NewClause(&toAddr).WithValue(transaction.Amount)

	// For VTHO transfers, we need to add contract call data
	if transaction.Asset == VTHO {
		// VTHO is transferred via the energy contract
		// This is a simplified implementation - in practice you'd need the proper contract ABI
		clause = clause.WithData([]byte{}) // Placeholder for VTHO transfer data
	}

	// Build the transaction
	thorTx := tx.NewBuilder(tx.TypeLegacy).
		ChainTag(chainTag).
		BlockRef(blockRef).
		Expiration(32). // 32 blocks expiration
		Gas(transaction.GasLimit.Uint64()).
		GasPriceCoef(0).
		Clause(clause).
		Build()

	// Sign the transaction
	signedTx, err := tx.Sign(thorTx, privateKey)
	if err != nil {
		return nil, NewBlockchainError(ErrTransactionFailed, "failed to sign transaction", err)
	}

	return signedTx, nil
}

func (c *Client) BroadcastTransaction(signedTx *tx.Transaction) (string, error) {
	response, err := c.thorClient.SendTransaction(signedTx)
	if err != nil {
		return "", NewNetworkError("failed to broadcast transaction", err)
	}

	return response.ID.String(), nil
}

func (c *Client) GetTransactionStatus(txID string) (*TransactionStatus, error) {
	txHash := common.HexToHash(txID)
	receipt, err := c.thorClient.TransactionReceipt(txHash)
	if err != nil {
		return nil, NewNetworkError("failed to get transaction status", err)
	}

	var status TransactionStatus
	if receipt.Reverted {
		status = StatusReverted
	} else {
		status = StatusConfirmed
	}

	return &status, nil
}

func (c *Client) WaitForConfirmation(txID string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return NewTimeoutError("waiting for transaction confirmation", timeout)
		case <-ticker.C:
			status, err := c.GetTransactionStatus(txID)
			if err != nil {
				continue
			}

			switch *status {
			case StatusConfirmed:
				return nil
			case StatusFailed, StatusReverted:
				return NewTransactionFailedError(txID, string(*status))
			}
		}
	}
}

func (c *Client) SendTransaction(from, to string, amount *big.Int, asset AssetType, privateKey *ecdsa.PrivateKey) (string, error) {
	// Build the transaction
	transaction, err := c.BuildTransaction(from, to, amount, asset)
	if err != nil {
		return "", err
	}

	// Sign the transaction
	signedTx, err := c.SignTransaction(transaction, privateKey)
	if err != nil {
		return "", err
	}

	// Broadcast the transaction
	txID, err := c.BroadcastTransaction(signedTx)
	if err != nil {
		return "", err
	}

	// Invalidate cache for the sender address
	c.InvalidateCache(from)

	return txID, nil
}

func (c *Client) Close() error {
	return nil
}
