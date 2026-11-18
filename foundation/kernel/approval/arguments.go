package approval

// SealedArguments separates the private input used to resume a call from the
// allowlisted fields shown on its approval card.
type SealedArguments struct {
	Ciphertext string         `json:"sealed"`
	Projection map[string]any `json:"projection"`
}
