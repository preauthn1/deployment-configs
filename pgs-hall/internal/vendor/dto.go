package vendor

// ServerGroup is dto for vendor api internal-api/merchant/{mid}/servers
type ServerGroup struct {
	Type     int     `json:"gameType"`
	TypeName string  `json:"gameTypeName"`
	Games    []*Game `json:"games"`
}

// Game is dto for vendor api internal-api/merchant/{mid}/servers
type Game struct {
	ID      	int       	`json:"gameID"`
	Name    	string		`json:"gameName"`
	Code    	string		`json:"gameCode"`
	Type    	int       	`json:"gameType"`
	TypeName	string		`json:"gameTypeName"`
	RTP			string		`json:"rtp"`
	IsShowExtraInfo	bool	`json:"isShowExtraInfo"`
	MaximumWin	string		`json:"maximumWin"`
	LineWays	string		`json:"lineWays"`
	ImageUrl	string		`json:"imageUrl"`
	Servers		[]*Server	`json:"servers"`
}

// Server is dto for vendor api internal-api/merchant/{mid}/servers
type Server struct {
	ID        int    `json:"serverID"`
	Name      string `json:"serverName"`
	FlagLevel int    `json:"flagLevel"`
	GameID    int    `json:"gameID"`
	GameType  int    `json:"gameType"`
}
