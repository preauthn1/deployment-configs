package game

// Group kind of games
type Group struct {
	Name  string  `json:"name"`
	Games []*Info `json:"games"`
}

// Info list which game provide
type Info struct {
	GID   int     `json:"gid"`
	Name  string  `json:"name"`
	Code  string  `json:"code"`
	Rooms []*Room `json:"rooms"`
}

// Room descirbe each room of games
type Room struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
