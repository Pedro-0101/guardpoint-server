package model

type ErrorResponse struct {
	Error string `json:"error" example:"mensagem descritiva do erro"`
}

type MessageResponse struct {
	Message string `json:"message" example:"operacao realizada com sucesso"`
}

type StatusResponse struct {
	Status string `json:"status" example:"ok"`
}
