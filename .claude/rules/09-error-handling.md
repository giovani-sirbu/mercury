# Error Handling Rules

## HTTP Handlers: Response Pattern

### CRITICAL: Every error gets a response with status code and error code
```go
adapter.Response(c, http.StatusBadRequest, "TRADE_ALREADY_EXIST")
adapter.Response(c, http.StatusNotFound, "EXCHANGE_NOT_FOUND")
adapter.Response(c, http.StatusInternalServerError, "COULD_NOT_CREATE_TRADE")
```

Error codes are SCREAMING_SNAKE_CASE strings. Never send raw error messages from Go's `err.Error()` to the client unless they are already formatted as error codes.

---

## Business Methods: (result, status, error) Pattern

### CRITICAL: Business methods return three values
```go
func (h *Handler) CreateTrade(...) (aggragates.Trades, int, error) {
    if trade.ID != 0 {
        return trade, http.StatusBadRequest, fmt.Errorf("TRADE_ALREADY_EXIST")
    }
    // ...
    if result.Error != nil {
        return trade, http.StatusInternalServerError, fmt.Errorf("COULD_NOT_CREATE_TRADE")
    }
    return trade, 0, nil  // success: status 0 means "no error status"
}
```

The HTTP handler uses the status code to respond. Consumers and jobs can ignore it.

---

## Universal Rules

### CRITICAL: Never swallow errors silently
Every `if err != nil` must either:
1. Return the error (with context)
2. Log the error and take corrective action
3. Log the error and continue (only if the error is truly non-fatal)

```go
// VIOLATION
result := h.server.DB.Create(&trade)
// no error check

// CORRECT
if result := h.server.DB.Create(&trade); result.Error != nil {
    return trade, http.StatusInternalServerError, fmt.Errorf("COULD_NOT_CREATE_TRADE")
}
```

### CRITICAL: Log errors with context
Use mercury's log package with function name and handler name:
```go
log.Error(err.Error(), "FunctionName", "HandlerName")
```

Examples:
```go
log.Error(err.Error(), "GetUserSession", "CreateTradeReq")
log.Error(result.Error.Error(), "DBQuery", "GetTrade")
```

### IMPORTANT: Error messages for users vs logs
- **User-facing:** Short, machine-readable error codes: `"TRADE_NOT_FOUND"`, `"INSUFFICIENT_FUNDS"`
- **Log-facing:** Detailed, includes context: function name, handler name, relevant IDs
- Never expose internal error details (stack traces, SQL errors) to the client

### IMPORTANT: Exchange error handling
Exchange API errors need special handling since they can be:
- String errors (invalid API key, permissions)
- Structured errors (insufficient funds with amounts)

```go
switch v := exchangeErr.(type) {
case string:
    return trade, http.StatusBadRequest, errors.New(v)
case InsufficientFundsResponse:
    errMsg := fmt.Sprintf("%s: Insufficient funds (%.8f %s)", v.Message, v.AvailableQuantity, v.Asset)
    return trade, http.StatusBadRequest, errors.New(errMsg)
}
```

### STANDARD: Validation errors use adapter.ValidationResponse
```go
err := helpers.ValidatePayload(payload)
if err != nil {
    adapter.ValidationResponse(c, err)
    return
}
```

This returns structured validation errors (field-level) to the client.
