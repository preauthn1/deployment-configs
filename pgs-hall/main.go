package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"

	// "sort"
	"strconv"
	"strings"
	"time"

	"example.invalid/gin-contrib/cors"
	"example.invalid/gin-gonic/gin"
	"example.invalid/spf13/cobra"
	"example.invalid/spf13/viper"

	"example.invalid/target/hall/internal/vendor"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringP("config", "c", "", "config file location")

	rootCmd.Flags().IntP("server.http.port", "p", 80, "Listen port for http")

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
}

// rootCmd for mc backend api for mc
var rootCmd = &cobra.Command{
	Use:               "target-hall",
	Short:             "target Hall",
	PersistentPreRunE: initViper,
	RunE: func(cmd *cobra.Command, args []string) error {
		title := viper.GetString("target.brand.title")
		brandName := viper.GetString("target.brand.name")
		brandLink := viper.GetString("target.brand.link")
		v := vendor.New(viper.GetString("target.api.uri"), viper.GetInt("target.merchant.id"), viper.GetString("target.merchant.api-key"))
		fixedUser := viper.GetString("target.merchant.fixed-user")
		isOpenAggFish := viper.GetBool("target.merchant.is-open-agg-fish")
		gameItemStyle := viper.GetInt("target.merchant.game-item-style")
		if gameItemStyle < 1 {
			gameItemStyle = 1
		}
		showDescription := viper.GetInt("target.merchant.show-description")
		r := gin.Default()
		corsConfig := cors.DefaultConfig()
		corsConfig.AllowAllOrigins = true
		r.Use(cors.New(corsConfig))
		r.LoadHTMLGlob("views/*.html")
		r.Static("static2", "./static2")
		r.Static("static", "./views/static")
		r.Static("fonts", "fonts")
		r.Static("img", "img")
		r.Static("js", "js")
		r.Static("media", "media")

		r.GET("", func(c *gin.Context) {
			c.HTML(http.StatusOK, "index.html", gin.H{
				"title":           title,
				"brandName":       brandName,
				"brandLink":       brandLink,
				"version":         os.Getenv("VERSION"),
				"fixedUser":       fixedUser,
				"gameItemStyle":   gameItemStyle,
				"showDescription": showDescription,
			})
		})

		r.GET("games", func(c *gin.Context) {
			//piriorities := []int{290088, 290089, 290087, 290083, 290082, 290081, 290080, 290075, 290074, 290069, 290062, 290054, 290049, 290046, 290041, 290039, 290038, 310002, 310003}
			//piriorities := []int{}

			lang := c.DefaultQuery("lang", "EN-US")
			games, err := v.GameList(lang)
			if err != nil {
				c.Error(err)
				c.String(http.StatusBadRequest, "Vendor API has error, %v", err)
				return
			}
			if isOpenAggFish {
				fishType := 7000
				fishIdx := -1
				for idx, game := range games {
					if game.Type == fishType {
						fishIdx = idx
					}
				}
				if fishIdx == -1 {
					fishIdx = len(games)
					games = append(games, &vendor.ServerGroup{
						Type:     fishType,
						TypeName: "Fishing",
						Games:    []*vendor.Game{},
					})
				}

				games[fishIdx].Games = append(games[fishIdx].Games, &vendor.Game{
					ID:       310002,
					Name:     "秦皇巡海",
					Code:     "agg-ht_emperorparade",
					Type:     7000,
					TypeName: "Fishing",
					ImageUrl: "https://example.invalid/game-icon/agg_ht-emperorparade.png",
					Servers: []*vendor.Server{
						&vendor.Server{
							ID:        310002,
							Name:      "秦皇巡海",
							FlagLevel: 1,
							GameID:    310002,
							GameType:  0,
						},
					},
				})
				games[fishIdx].Games = append(games[fishIdx].Games, &vendor.Game{
					ID:       310003,
					Name:     "鴻運金牛",
					Code:     "agg-ht_goldenbull",
					Type:     7000,
					TypeName: "Fishing",
					ImageUrl: "https://example.invalid/game-icon/agg_ht-goldenbull.png",
					Servers: []*vendor.Server{
						&vendor.Server{
							ID:        310003,
							Name:      "鴻運金牛",
							FlagLevel: 1,
							GameID:    310003,
							GameType:  0,
						},
					},
				})
			}

			// ----------------
			// 2023/10/19 以API排序結果為準 不在這邊處理排序
			// ----------------

			// for _, game := range games {
			// 	sort.Slice(game.Games, func(i, j int) bool {

			// 		//game.Games[i].ID in piriorities
			// 		isIPiriority := false
			// 		isJPiriority := false
			// 		for _, v := range piriorities {
			// 			if game.Games[i].ID == v {
			// 				isIPiriority = true
			// 			}
			// 		}

			// 		for _, v := range piriorities {
			// 			if game.Games[j].ID == v {
			// 				isJPiriority = true
			// 			}
			// 		}
			// 		if isIPiriority && !isJPiriority {
			// 			return true
			// 		} else if !isIPiriority && isJPiriority {
			// 			return false
			// 		} else {
			// 			return game.Games[i].ID > game.Games[j].ID
			// 		}
			// 	})
			// }

			c.IndentedJSON(http.StatusOK, games)
		})

		// 123456
		r.GET("open", func(c *gin.Context) {

			gid := c.Query("gid")
			if gid == "310001" {
				c.Redirect(http.StatusMovedPermanently, "https://example.invalid/index.html")
			} else if gid == "310002" {
				gameurl := fmt.Sprintf(
					"https://example.invalid/autojoin.html?gameid=%s&language=%s&return_url=%s",
					"tfe",
					langReplace(c.DefaultQuery("lang", "EN-US")),
					"https://example.invalid/",
				)
				log.Printf("gameurl: %s", gameurl)
				c.Redirect(http.StatusMovedPermanently, gameurl)
			} else if gid == "310003" {
				gameurl := fmt.Sprintf(
					"https://example.invalid/autojoin.html?gameid=%s&language=%s&return_url=%s",
					"bull",
					langReplace(c.DefaultQuery("lang", "EN-US")),
					"https://example.invalid/",
				)
				log.Printf("gameurl: %s", gameurl)
				c.Redirect(http.StatusMovedPermanently, gameurl)
			}
			rid := c.Query("rid")
			sid := c.Query("sid")
			lang := c.DefaultQuery("lang", "EN-US")
			user := c.DefaultQuery("user", "demo-000")

			_gid, _ := strconv.ParseInt(gid, 0, 64)
			_rid, _ := strconv.ParseInt(rid, 0, 64)
			_sid, _ := strconv.ParseInt(sid, 0, 64)

			isSingleWallet := viper.GetBool("target.merchant.is-single-wallet")
			url := ""
			var err error = nil
			if isSingleWallet {
				url, err = v.SingleWalletLaunchGame(int(_gid), int(_sid), int(_rid), lang, user)
			} else {
				url, err = v.LaunchGame(int(_gid), int(_sid), int(_rid), lang, user)
			}

			if err != nil {
				c.Error(err)
				c.String(http.StatusBadRequest, "Vendor API has error, %v", err)
				return
			}

			c.Redirect(http.StatusMovedPermanently, url)
		})

		r.GET("try-game", func(c *gin.Context) {
			gid := c.Query("gid")
			if gid == "310001" {
				c.Redirect(http.StatusMovedPermanently, "https://example.invalid/index.html")
			} else if gid == "310002" {
				gameurl := fmt.Sprintf(
					"https://example.invalid/autojoin.html?gameid=%s&language=%s&return_url=%s",
					"tfe",
					langReplace(c.DefaultQuery("lang", "EN-US")),
					"https://example.invalid/",
				)
				log.Printf("gameurl: %s", gameurl)
				c.Redirect(http.StatusMovedPermanently, gameurl)
			} else if gid == "310003" {
				gameurl := fmt.Sprintf(
					"https://example.invalid/autojoin.html?gameid=%s&language=%s&return_url=%s",
					"bull",
					langReplace(c.DefaultQuery("lang", "EN-US")),
					"https://example.invalid/",
				)
				log.Printf("gameurl: %s", gameurl)
				c.Redirect(http.StatusMovedPermanently, gameurl)
			}
			rid := c.Query("rid")
			sid := c.Query("sid")
			lang := c.DefaultQuery("lang", "EN-US")

			//generate timestamp
			timestamp := time.Now().UnixNano() / 1000000
			user := c.DefaultQuery("user", fmt.Sprintf("demo-%d", timestamp))

			_gid, _ := strconv.ParseInt(gid, 0, 64)
			_rid, _ := strconv.ParseInt(rid, 0, 64)
			_sid, _ := strconv.ParseInt(sid, 0, 64)

			isSingleWallet := viper.GetBool("target.merchant.is-single-wallet")
			url := ""
			var err error = nil
			if isSingleWallet {
				url, err = v.SingleWalletLaunchGame(int(_gid), int(_sid), int(_rid), lang, user)
			} else {
				url, err = v.LaunchGame(int(_gid), int(_sid), int(_rid), lang, user)
			}

			if err != nil {
				c.Error(err)
				c.String(http.StatusBadRequest, "Vendor API has error, %v", err)
				return
			}

			c.Redirect(http.StatusMovedPermanently, url)
		})

		serv := &http.Server{
			Addr:    fmt.Sprintf(":%d", viper.GetInt("server.http.port")),
			Handler: r,
		}

		if err := serv.ListenAndServe(); err != nil {
			return err
		}

		return nil
	},
}

func langReplace(lang string) string {
	lang = strings.Replace(lang, "-", "_", -1)
	lang = strings.Replace(lang, strings.ToLower("zh_TW"), "zh_TW", -1)
	lang = strings.Replace(lang, strings.ToLower("zh_CN"), "zh_CN", -1)
	lang = strings.Replace(lang, strings.ToLower("en_US"), "en_US", -1)
	lang = strings.Replace(lang, strings.ToLower("th_TH"), "th_TH", -1)
	lang = strings.Replace(lang, strings.ToLower("vi_VN"), "vi_VN", -1)
	lang = strings.Replace(lang, strings.ToLower("ms_MY"), "ms_MY", -1)
	lang = strings.Replace(lang, strings.ToLower("pt_PT"), "pt_PT", -1)
	return lang
}
func initViper(cmd *cobra.Command, args []string) (err error) {
	var appName string
	if cmd.Parent() != nil {
		appName += cmd.Parent().Use
		appName += "."
	}

	appName += cmd.Use

	var cfg string

	if f := cmd.Flags().Lookup("config"); f != nil {
		cfg = f.Value.String()
	}

	if cfg != "" {
		viper.SetConfigFile(cfg)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./configs")
		viper.AddConfigPath("$HOME/." + appName)
		viper.AddConfigPath("$HOME")
		viper.AddConfigPath("/")
	}

	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		return err
	}

	viper.SetEnvPrefix(appName)
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	cmd.Println("Using config file:", viper.ConfigFileUsed())

	return nil
}
