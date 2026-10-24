package game

import (
	"log"
	"strings"

	"example.invalid/BurntSushi/toml"
	"example.invalid/nicksnyder/go-i18n/v2/i18n"
	"example.invalid/spf13/viper"
	"example.invalid/x/text/language"
)

var bundle *i18n.Bundle

func init() {
	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	bundle.MustLoadMessageFile("i18n/en-us.toml")
	bundle.MustLoadMessageFile("i18n/zh-tw.toml")
	bundle.MustLoadMessageFile("i18n/zh-cn.toml")
}

// List return provide games
func List(lang string) []*Group {

	grps := make([]*Group, 0)

	if err := viper.UnmarshalKey("target.groups", &grps); err != nil {
		log.Printf("Unmarshal Game List Error, %v", err)
	}

	lang = strings.ToLower(lang)
	localizer := i18n.NewLocalizer(bundle, lang)

	for _, grp := range grps {
		if val, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: grp.Name}); err != nil {
			log.Printf("Localize Error, %v\n", err)
		} else {
			grp.Name = val
		}
		for _, game := range grp.Games {
			if val, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: game.Code}); err != nil {
				log.Printf("Localize Error, %v\n", err)
				game.Name = game.Code
			} else {
				game.Name = val
			}
			for _, room := range game.Rooms {
				if val, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: room.Name}); err != nil {
					log.Printf("Localize Error, %v\n", err)
				} else {
					room.Name = val
				}
			}
		}
	}

	return grps
}
