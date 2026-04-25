package assembly

func validTestConfig() Config {
	return Config{
		Gateway:        "https://gateway.example.com",
		APIKey:         "test-key",
		SidecarAddress: "127.0.0.1:50051",
	}
}
