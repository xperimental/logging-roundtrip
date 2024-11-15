package loki

type Message struct {
	Streams []Stream `json:"streams"`
}

type Stream struct {
	Labels map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}
