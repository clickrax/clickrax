package health

type Check struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type Report struct {
	Checks []Check `json:"checks"`
	OK     bool    `json:"ok"`
}
