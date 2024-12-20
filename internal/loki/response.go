package loki

type Response struct {
	Status string       `json:"status"`
	Data   StreamResult `json:"data"`
}

type StreamResult struct {
	Type   string   `json:"resultType"`
	Result []Stream `json:"result"`
}
