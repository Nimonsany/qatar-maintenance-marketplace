package authboundary

import (
	"errors"
	"qatar-maintenance/backend/internal/marketplace"
	"testing"
)

func TestRolePolicyTable(t *testing.T) {
	rules := map[Action][]marketplace.Role{
		CreateRequest: {marketplace.Customer},
		ListRequests:  {marketplace.Customer, marketplace.Admin},
		ViewRequest:   {marketplace.Customer, marketplace.Worker, marketplace.Company, marketplace.Admin},
		QuoteRequest:  {marketplace.Worker, marketplace.Company},
		AcceptQuote:   {marketplace.Customer},
		AssignWorker:  {marketplace.Admin},
		StartWork:     {marketplace.Worker, marketplace.Company},
		CompleteWork:  {marketplace.Worker, marketplace.Company},
		CancelRequest: {marketplace.Customer, marketplace.Admin},
	}
	for action, allowed := range rules {
		for _, role := range []marketplace.Role{marketplace.Customer, marketplace.Worker, marketplace.Company, marketplace.Admin, "superadmin", ""} {
			want := false
			for _, a := range allowed {
				if role == a {
					want = true
				}
			}
			err := PermitRole(role, action)
			if want && err != nil {
				t.Errorf("%s/%s denied %v", action, role, err)
			}
			if !want && !errors.Is(err, ErrForbidden) {
				t.Errorf("%s/%s permitted", action, role)
			}
		}
	}
	if err := PermitRole(marketplace.Admin, "unknown-action"); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
}
