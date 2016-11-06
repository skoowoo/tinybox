package proto

const (
	Success = "success"
	Failed  = "failed"
)

type Response struct {
	Status string `json:"status"`
	Desc   string `json:"desc"`
}

type ExecRequest struct {
	Path string   `json:"path"`
	Argv []string `json:"argv"`
}

type ExecResponse struct {
	Response
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}
