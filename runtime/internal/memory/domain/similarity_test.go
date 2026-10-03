package domain

import "testing"

func TestNearDuplicate(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{
			"punctuation and case only",
			"The integration suite requires a local Redis instance on localhost before it can pass.",
			"the integration suite requires a local redis instance on localhost before it can pass",
			true,
		},
		{
			"one word of a long claim reworded",
			"the integration test suite requires a running local redis instance on the default localhost port before the continuous integration build for any pull request can pass",
			"the integration test suite requires a running local redis instance on the default localhost port before the continuous integration build for any pull request will pass",
			true,
		},
		{
			"a changed number is a different fact",
			"outbound webhook delivery is retried at most 3 times before the job is marked failed permanently",
			"outbound webhook delivery is retried at most 5 times before the job is marked failed permanently",
			false,
		},
		{
			"a negation reverses the claim",
			"the staging environment is enabled for the nightly integration run on this repository",
			"the staging environment is not enabled for the nightly integration run on this repository",
			false,
		},
		{
			"a changed value of a short claim",
			"the production database is hosted in the frankfurt region",
			"the production database is hosted in the virginia region",
			false,
		},
		{
			"short claims are never fuzzy matched",
			"redis runs on port",
			"redis runs on ports",
			false,
		},
		{
			"unrelated",
			"the release pipeline deploys through the make ship target on every tagged commit here",
			"the linter configuration forbids unused imports across every package in the runtime module",
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NearDuplicate(tc.a, tc.b); got != tc.want {
				t.Errorf("NearDuplicate = %v, want %v", got, tc.want)
			}
		})
	}
}
