package cli

import (
	"github.com/alimtvnetwork/yt-manage/internal/config"
)

// withDefaultChannel applies the remembered default_channel
// setting (spec 29, item 78) when a command was not given an
// explicit --channel. An explicit flag always wins; a missing or
// unreadable config simply leaves the value alone.
func withDefaultChannel(channelID uint) uint {
	if channelID != 0 {
		return channelID
	}
	home, err := config.HomeDir()
	if err != nil {
		return channelID
	}
	settings, err := config.LoadSettings(home)
	if err != nil || settings.DefaultChannel <= 0 {
		return channelID
	}
	return uint(settings.DefaultChannel)
}

// withDefaultAccount applies the remembered default_account
// setting when a command was not given an explicit account.
func withDefaultAccount(accountID uint) uint {
	if accountID != 0 {
		return accountID
	}
	home, err := config.HomeDir()
	if err != nil {
		return accountID
	}
	settings, err := config.LoadSettings(home)
	if err != nil || settings.DefaultAccount <= 0 {
		return accountID
	}
	return uint(settings.DefaultAccount)
}
