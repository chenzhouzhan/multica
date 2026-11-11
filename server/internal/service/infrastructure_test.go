package service

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/integrations/transwarp"
)

func TestInfrastructureRestartGrantChecksTheEntireResourceIdentity(t *testing.T) {
	grant := InfrastructureGrant{ClusterID: 7, ServiceID: 8, RoleID: 9}
	original := transwarp.Role{ID: 9, ServiceID: 8, Node: transwarp.Node{ClusterID: 7}, Installed: true}
	cases := []struct {
		name string
		role transwarp.Role
		want bool
	}{
		{"exact role", original, true},
		{"other cluster", transwarp.Role{ID: 9, ServiceID: 8, Node: transwarp.Node{ClusterID: 70}, Installed: true}, false},
		{"other service", transwarp.Role{ID: 9, ServiceID: 80, Node: transwarp.Node{ClusterID: 7}, Installed: true}, false},
		{"other role", transwarp.Role{ID: 90, ServiceID: 8, Node: transwarp.Node{ClusterID: 7}, Installed: true}, false},
		{"uninstalled role", transwarp.Role{ID: 9, ServiceID: 8, Node: transwarp.Node{ClusterID: 7}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := grantMatchesRole(grant, tc.role); got != tc.want {
				t.Fatalf("allowed=%v want=%v", got, tc.want)
			}
		})
	}
}
