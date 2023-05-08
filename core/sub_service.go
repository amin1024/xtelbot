package core

import (
	"fmt"
	"github.com/amin1024/xtelbot/core/repo"
	"strings"
)

func NewSubService() *SubService {
	return &SubService{
		nodesService: NewNodesService(),
	}
}

type SubService struct {
	nodesService *NodesService
}

// GenerateUserSub requests user's sub-content from xNodes and merge them together
func (s *SubService) GenerateUserSub(token string) (string, error) {
	u, err := repo.GetUserByToken(token)
	if err != nil {
		return "", err
	}
	subs := s.nodesService.GetSubs(u)
	if len(subs) == 0 {
		return "", fmt.Errorf("unable to generate sub for token=%s", token)
	}

	// Join the sub links
	return strings.Join(subs, "\n-----\n"), nil
}
