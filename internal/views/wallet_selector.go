package views

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"rhystmorgan/veWallet/internal/storage"
	"rhystmorgan/veWallet/internal/utils"
)

type WalletSelectorModel struct {
	wallets  []storage.EncryptedWallet
	cursor   int
	selected int
}

func NewWalletSelectorModel(wallets []storage.EncryptedWallet) *WalletSelectorModel {
	return &WalletSelectorModel{
		wallets:  wallets,
		cursor:   0,
		selected: -1,
	}
}

func (m WalletSelectorModel) Init() tea.Cmd {
	return nil
}

func (m WalletSelectorModel) Update(msg tea.Msg) (WalletSelectorModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.wallets)+2 {
				m.cursor++
			}
		case "enter", " ":
			if m.cursor < len(m.wallets) {
				m.selected = m.cursor
			} else if m.cursor == len(m.wallets) {
				return m, NavigateTo(ViewWalletCreate, nil)
			} else if m.cursor == len(m.wallets)+1 {
				return m, NavigateTo(ViewWalletImport, nil)
			}
		}
	}
	return m, nil
}

func (m WalletSelectorModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Bold(true).
		Padding(1, 0)

	itemStyle := lipgloss.NewStyle().
		Padding(0, 2)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Green)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 2)

	var content string
	content += titleStyle.Render("VeTerm - VeChain Wallet") + "\n\n"

	if len(m.wallets) == 0 {
		content += lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Subtext0)).
			Render("No wallets found. Create or import a wallet to get started.") + "\n\n"
	} else {
		content += lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Text)).
			Render("Select a wallet:") + "\n\n"

		for i, wallet := range m.wallets {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
			}

			style := itemStyle
			if m.cursor == i {
				style = selectedStyle
			}

			content += style.Render(fmt.Sprintf("%s %s (%s)", cursor, wallet.Name, wallet.Address[:10]+"...")) + "\n"
		}
		content += "\n"
	}

	cursor := " "
	if m.cursor == len(m.wallets) {
		cursor = ">"
	}
	style := itemStyle
	if m.cursor == len(m.wallets) {
		style = selectedStyle
	}
	content += style.Render(fmt.Sprintf("%s Create New Wallet", cursor)) + "\n"

	cursor = " "
	if m.cursor == len(m.wallets)+1 {
		cursor = ">"
	}
	style = itemStyle
	if m.cursor == len(m.wallets)+1 {
		style = selectedStyle
	}
	content += style.Render(fmt.Sprintf("%s Import Wallet", cursor)) + "\n\n"

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Subtext0)).
		Italic(true)

	content += helpStyle.Render("Use ↑/↓ to navigate, Enter to select, q to quit")

	return content
}
