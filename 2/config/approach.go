package config

type Approach int

const (
	HTTPApproach Approach = iota
	FastHTTPApproach
	GRPCApproach
)
