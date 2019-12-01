package main

type registerRequest struct {
	Name string `json:"name"`
}

type registerResponse struct {
	AuthenticationToken string `json:"authentication_token"`
}
