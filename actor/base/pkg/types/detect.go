package types

type DetectFile struct {
	Branches map[string]string `json:"branches"`
	Tags     map[string]string `json:"tags"`
}
