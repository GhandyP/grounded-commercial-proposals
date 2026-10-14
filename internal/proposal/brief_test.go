package proposal

import "testing"

func TestBriefReadinessReportsNothingForACompleteBrief(t *testing.T) {
	brief := Brief{
		Customer:    "Acme",
		Needs:       []string{"cut the manual reconciliation work"},
		Scope:       []string{"billing migration"},
		Constraints: []string{"must land before Q3 close"},
	}
	if findings := BriefReadiness(brief); len(findings) != 0 {
		t.Fatalf("BriefReadiness() = %+v, want no findings", findings)
	}
}

func TestBriefReadinessReportsEveryMissingInput(t *testing.T) {
	tests := []struct {
		name     string
		brief    Brief
		want     []string
		subjects []string
	}{
		{
			name:     "no customer",
			brief:    Brief{Needs: []string{"a"}, Scope: []string{"b"}, Constraints: []string{"c"}},
			want:     []string{CodeBriefCustomerMissing},
			subjects: []string{"brief.customer"},
		},
		{
			name:     "blank customer",
			brief:    Brief{Customer: "   ", Needs: []string{"a"}, Scope: []string{"b"}, Constraints: []string{"c"}},
			want:     []string{CodeBriefCustomerMissing},
			subjects: []string{"brief.customer"},
		},
		{
			name:     "no needs",
			brief:    Brief{Customer: "Acme", Scope: []string{"b"}, Constraints: []string{"c"}},
			want:     []string{CodeBriefNeedMissing},
			subjects: []string{"brief.needs"},
		},
		{
			name:     "needs entries are all blank",
			brief:    Brief{Customer: "Acme", Needs: []string{" ", "\t"}, Scope: []string{"b"}, Constraints: []string{"c"}},
			want:     []string{CodeBriefNeedMissing},
			subjects: []string{"brief.needs"},
		},
		{
			name:     "no scope",
			brief:    Brief{Customer: "Acme", Needs: []string{"a"}, Constraints: []string{"c"}},
			want:     []string{CodeBriefScopeMissing},
			subjects: []string{"brief.scope"},
		},
		{
			name:     "no constraints",
			brief:    Brief{Customer: "Acme", Needs: []string{"a"}, Scope: []string{"b"}},
			want:     []string{CodeBriefConstraintMissing},
			subjects: []string{"brief.constraints"},
		},
		{
			name:  "an empty brief is missing everything",
			brief: Brief{},
			want: []string{
				CodeBriefCustomerMissing,
				CodeBriefNeedMissing,
				CodeBriefScopeMissing,
				CodeBriefConstraintMissing,
			},
			subjects: []string{"brief.customer", "brief.needs", "brief.scope", "brief.constraints"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			findings := BriefReadiness(tc.brief)
			if len(findings) != len(tc.want) {
				t.Fatalf("BriefReadiness() = %+v, want %d findings", findings, len(tc.want))
			}
			for i, code := range tc.want {
				if findings[i].Code != code {
					t.Fatalf("finding %d = %q, want %q", i, findings[i].Code, code)
				}
				if findings[i].Subject != tc.subjects[i] {
					t.Fatalf("finding %d subject = %q, want %q", i, findings[i].Subject, tc.subjects[i])
				}
				if findings[i].Message == "" {
					t.Fatalf("finding %d (%s) has no message", i, code)
				}
			}
		})
	}
}

func TestBriefReadinessSeveritySeparatesScopeFromContent(t *testing.T) {
	// A missing customer blocks the draft: every record is scoped to a customer.
	missingCustomer := BriefReadiness(Brief{})
	if sev := missingCustomer[0].Severity; sev != SeverityError {
		t.Fatalf("missing customer severity = %q, want %q", sev, SeverityError)
	}
	// A missing need, scope or constraint is visible but does not block.
	for _, f := range missingCustomer[1:] {
		if f.Severity != SeverityWarning {
			t.Fatalf("finding %s severity = %q, want %q", f.Code, f.Severity, SeverityWarning)
		}
	}
}

func TestAssumptionsFromBriefNamesEveryMissingInput(t *testing.T) {
	assumptions := AssumptionsFromBrief(Brief{})
	if len(assumptions) != 4 {
		t.Fatalf("AssumptionsFromBrief() = %+v, want four assumptions", assumptions)
	}
	wantIDs := []string{"A-1", "A-2", "A-3", "A-4"}
	wantBasis := []string{"brief.customer is empty", "brief.needs is empty", "brief.scope is empty", "brief.constraints is empty"}
	for i, a := range assumptions {
		if a.ID != wantIDs[i] {
			t.Fatalf("assumption %d id = %q, want %q", i, a.ID, wantIDs[i])
		}
		if a.Basis != wantBasis[i] {
			t.Fatalf("assumption %d basis = %q, want %q", i, a.Basis, wantBasis[i])
		}
		if !a.NeedsConfirmation {
			t.Fatalf("assumption %d does not need confirmation: %+v", i, a)
		}
		if NormalizeText(a.Text) == "" {
			t.Fatalf("assumption %d has no text", i)
		}
	}
}

func TestAssumptionsFromBriefSkipsWhatTheBriefStates(t *testing.T) {
	assumptions := AssumptionsFromBrief(Brief{
		Customer:    "Acme",
		Needs:       []string{"cut the manual reconciliation work"},
		Scope:       []string{"billing migration"},
		Constraints: []string{"must land before Q3 close"},
	})
	if len(assumptions) != 0 {
		t.Fatalf("AssumptionsFromBrief() = %+v, want no assumptions", assumptions)
	}

	partial := AssumptionsFromBrief(Brief{Customer: "Acme", Needs: []string{"cut cost"}})
	if len(partial) != 2 {
		t.Fatalf("AssumptionsFromBrief() = %+v, want two assumptions", partial)
	}
	if partial[0].Basis != "brief.scope is empty" || partial[1].Basis != "brief.constraints is empty" {
		t.Fatalf("assumptions = %+v, want scope then constraints", partial)
	}
}
