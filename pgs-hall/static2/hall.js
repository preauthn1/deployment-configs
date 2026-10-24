let app = new Vue({
    el: "#app",
    delimiters: ["${", "}"],
    data: {
        user: "",
        games: {},
        lang: "EN-US",
        langs: {
            English: "EN-US",
            // 繁體中文: "ZH-TW",
            简体中文: "ZH-CN",
        },
        currentGroupTypeName: 'All'
    },
    methods: {
        reload: function(lang) {
            $this = this
            fetch("/games?lang=" + lang)
            .then(res => { return res.json() })
            .then(data => { 
                // for fishing king
                data.forEach(element => {
                    if(element.gameType == 7000) {
                        element.games.forEach(game=>{
                            if(game.gameID == 310001) {
                                game.gameName = "Fishing King";
                                game.imageUrl = "https://example.invalid/game-icon/aggfish_fishingking.png";
                                //console.log(game)
                            }
                            //console.log(game)
                        })
                    }
                });
                $this.games = data 
            })
        },
        open: function(gid, sid) {
            let url = window.location.protocol + "//" + window.location.host + "/open?gid=" + gid + "&sid=" + sid  + "&user=" + this.user + "&lang=" + this.lang + "&ts=" + Date.now()
            console.log(url)
            window.open(url)
        },
        changeGroup: function(text) {
            this.currentGroupTypeName = text;
            $('.navbar-collapse').collapse('hide');
        },
    },
    mounted: function() {
        if(fixedUser != "") {
            this.user = fixedUser;
        } else {
            this.user = "demo-" + Date.now()
        }
        var hashValue = decodeURIComponent(location.hash.replace("#", ""));
        this.currentGroupTypeName = hashValue?hashValue:"All";
        this.reload(this.lang)
    }
})