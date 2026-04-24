package assembly

import (
	"errors"
	"testing"
)

func TestValidateConfig(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		cfg     Config
		wantErr error
	}{
		{
			name: "missing gateway",
			cfg: Config{
				APIKey:         "test-key",
				SidecarAddress: "127.0.0.1:50051",
			},
			wantErr: ErrInvalidGateway,
		},
		{
			name: "missing api key",
			cfg: Config{
				Gateway:        "https://gateway.example.com",
				SidecarAddress: "127.0.0.1:50051",
			},
			wantErr: ErrInvalidAPIKey,
		},
		{
			name:    "valid config",
			cfg:     validTestConfig(),
			wantErr: nil,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateConfig(tc.cfg)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected error %v, got %v", tc.wantErr, err)
			}
		})
	}
}
