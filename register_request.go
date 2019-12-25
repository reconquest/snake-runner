package main

//go:generate gonstructor -type registerRequest
type registerRequest struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

type registerResponse struct {
	AuthenticationToken string `json:"authentication_token"`
}