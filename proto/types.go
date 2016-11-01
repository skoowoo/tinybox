package proto

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
	Stdout []byte `json:"stdout"`
	Stderr []byte `json:"stderr"`
}
