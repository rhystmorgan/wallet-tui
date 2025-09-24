package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"rhystmorgan/veWallet/internal/models"
)

const (
	appDir       = ".veterm"
	walletsFile  = "wallets.json"
	contactsFile = "contacts.json"
	configFile   = "config.json"
)

type Storage struct {
	dataDir string
}

type WalletStorage struct {
	Wallets []EncryptedWallet `json:"wallets"`
}

type EncryptedWallet struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Address   string         `json:"address"`
	CreatedAt string         `json:"created_at"`
	Data      *EncryptedData `json:"data"`
}

type Config struct {
	DefaultWallet string `json:"default_wallet,omitempty"`
	Network       string `json:"network"`
	Theme         string `json:"theme"`
}

func NewStorage() (*Storage, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	dataDir := filepath.Join(homeDir, appDir)
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	return &Storage{dataDir: dataDir}, nil
}

func (s *Storage) SaveWallet(wallet *models.Wallet, password string) error {
	walletData, err := json.Marshal(wallet)
	if err != nil {
		return fmt.Errorf("failed to marshal wallet: %w", err)
	}

	encryptedData, err := Encrypt(walletData, password)
	if err != nil {
		return fmt.Errorf("failed to encrypt wallet: %w", err)
	}

	encWallet := EncryptedWallet{
		ID:        wallet.ID,
		Name:      wallet.Name,
		Address:   wallet.Address,
		CreatedAt: wallet.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Data:      encryptedData,
	}

	storage, err := s.loadWalletStorage()
	if err != nil {
		return err
	}

	for i, existing := range storage.Wallets {
		if existing.ID == wallet.ID {
			storage.Wallets[i] = encWallet
			return s.saveWalletStorage(storage)
		}
	}

	storage.Wallets = append(storage.Wallets, encWallet)
	return s.saveWalletStorage(storage)
}

func (s *Storage) LoadWallet(id, password string) (*models.Wallet, error) {
	storage, err := s.loadWalletStorage()
	if err != nil {
		return nil, err
	}

	for _, encWallet := range storage.Wallets {
		if encWallet.ID == id {
			walletData, err := Decrypt(encWallet.Data, password)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt wallet: %w", err)
			}

			var wallet models.Wallet
			if err := json.Unmarshal(walletData, &wallet); err != nil {
				return nil, fmt.Errorf("failed to unmarshal wallet: %w", err)
			}

			return &wallet, nil
		}
	}

	return nil, fmt.Errorf("wallet not found")
}

func (s *Storage) ListWallets() ([]EncryptedWallet, error) {
	storage, err := s.loadWalletStorage()
	if err != nil {
		return nil, err
	}
	return storage.Wallets, nil
}

func (s *Storage) DeleteWallet(id string) error {
	storage, err := s.loadWalletStorage()
	if err != nil {
		return err
	}

	for i, wallet := range storage.Wallets {
		if wallet.ID == id {
			storage.Wallets = append(storage.Wallets[:i], storage.Wallets[i+1:]...)
			return s.saveWalletStorage(storage)
		}
	}

	return fmt.Errorf("wallet not found")
}

func (s *Storage) SaveContacts(contacts *models.ContactList) error {
	data, err := json.MarshalIndent(contacts, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal contacts: %w", err)
	}

	filePath := filepath.Join(s.dataDir, contactsFile)
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write contacts file: %w", err)
	}

	return nil
}

func (s *Storage) LoadContacts() (*models.ContactList, error) {
	filePath := filepath.Join(s.dataDir, contactsFile)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &models.ContactList{Contacts: []models.Contact{}}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read contacts file: %w", err)
	}

	var contacts models.ContactList
	if err := json.Unmarshal(data, &contacts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contacts: %w", err)
	}

	return &contacts, nil
}

func (s *Storage) SaveConfig(config *Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	filePath := filepath.Join(s.dataDir, configFile)
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (s *Storage) LoadConfig() (*Config, error) {
	filePath := filepath.Join(s.dataDir, configFile)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		defaultConfig := &Config{
			Network: "mainnet",
			Theme:   "catppuccin",
		}
		return defaultConfig, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

func (s *Storage) loadWalletStorage() (*WalletStorage, error) {
	filePath := filepath.Join(s.dataDir, walletsFile)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &WalletStorage{Wallets: []EncryptedWallet{}}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read wallets file: %w", err)
	}

	var storage WalletStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal wallet storage: %w", err)
	}

	return &storage, nil
}

func (s *Storage) saveWalletStorage(storage *WalletStorage) error {
	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal wallet storage: %w", err)
	}

	filePath := filepath.Join(s.dataDir, walletsFile)
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write wallets file: %w", err)
	}

	return nil
}
