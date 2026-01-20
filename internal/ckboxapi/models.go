package ckboxapi

type CkboxEnv struct {
	Name string `json:"name"`
	Id   string `json:"id"`
}

type CkboxEnvRespBody struct {
	Items []CkboxEnv `json:"items"`
}

type CkboxEnvCreateReqBody struct {
	Name   string `json:"name"`
	Region string `json:"region"`
}

type CkboxAccesKey struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type CkboxReadAccesKeyRespBody struct {
	Items []CkboxAccesKey `json:"items"`
}

type CkboxCreateAccessKeyReqBody struct {
	Name string `json:"name"`
}
