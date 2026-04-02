package service

// Config holds the external service base URLs needed by shopping-list logic.
type Config struct {
	RecipeURL     string
	PantryURL     string
	DictionaryURL string
}

// Service stores shared dependencies for future shopping-list operations.
type Service struct {
	config Config
}

// New constructs the service scaffold without generation behavior.
func New(config Config) *Service {
	return &Service{config: config}
}

// Config returns the configured upstream dependency URLs.
func (s *Service) Config() Config {
	return s.config
}
