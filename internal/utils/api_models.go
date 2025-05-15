package utils 


type APIResponse[T any] struct {
    Status  string `json:"status"`  // e.g., "success", "error"
    Message string `json:"message"` // e.g., "Operation successful"
    Data    T      `json:"data"`    // Any data payload
}


