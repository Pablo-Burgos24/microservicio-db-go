package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"microservicio-db-go/domain"
)

// MockHealthRepository es un mock de domain.HealthRepository para tests
type MockHealthRepository struct {
	mock.Mock
}

func (m *MockHealthRepository) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestHealthUseCase_IsHealthy(t *testing.T) {
	tests := []struct {
		name       string
		pingError  error
		expectedOK bool
	}{
		{
			name:       "ping exitoso",
			pingError:  nil,
			expectedOK: true,
		},
		{
			name:       "ping falla",
			pingError:  assert.AnError,
			expectedOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockHealthRepository)
			uc := NewHealthUseCase(mockRepo)

			ctx := context.Background()
			mockRepo.On("Ping", ctx).Return(tt.pingError)

			result := uc.IsHealthy(ctx)

			assert.Equal(t, tt.expectedOK, result)
			mockRepo.AssertExpectations(t)
		})
	}
}