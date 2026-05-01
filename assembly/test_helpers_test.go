package assembly

func validTestOptions() []Option {
	return []Option{
		WithGatewayURL("https://gateway.example.com"),
		WithAPIKey("test-key"),
		withSidecarAddress("127.0.0.1:50051"),
	}
}
