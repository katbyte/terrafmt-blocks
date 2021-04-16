package upgrade012

import (
	"context"
	"strings"
	"testing"

	"github.com/katbyte/terrafmt/lib/common"
)

func TestFmtVerbBlock(t *testing.T) {
	tests := []struct {
		name     string
		block    string
		expected string
		error    bool
	}{
		{
			name: "noverbs",
			block: `data "google_dns_managed_zone" "qa" {
	name	= "${google_dns_managed_zone.foo.name}"
}
`,
			expected: `data "google_dns_managed_zone" "qa" {
  name = google_dns_managed_zone.foo.name
}
`,
		},

		{
			name: "resource_name",
			block: `data "google_compute_address" "%s" {
	name = "${google_compute_address.%s.name}"
}
`,
			expected: `data "google_compute_address" "%s" {
  name = google_compute_address.%s.name
}
`,
		},

		//todo nested or forloop with letters?
		{
			name: "bareverb",
			block: `%s
    %s
	%s

%d
    %d

%t
    %t

%q
    %q

%f
    %f

%g
    %g

data "google_dns_managed_zone" "qa" {
  name = "${google_dns_managed_zone.foo.name}"
}
`,
			expected: `%s
    %s
	%s

%d
    %d

%t
    %t

%q
    %q

%f
    %f

%g
    %g

data "google_dns_managed_zone" "qa" {
  name = google_dns_managed_zone.foo.name
}
`,
		},

		{
			name: "bareverb-positional",
			block: `%[1]s
    %[7]s
	%[77]s

%[7]d
    %[7]d

%[42]t
    %[1]t

%[7]q
    %[77]q

%[7]f
    %[77]f

%[1]g
    %[2]g

data "google_dns_managed_zone" "qa" {
  name = "${google_dns_managed_zone.foo.name}"
}
`,
			expected: `%[1]s
    %[7]s
	%[77]s

%[7]d
    %[7]d

%[42]t
    %[1]t

%[7]q
    %[77]q

%[7]f
    %[77]f

%[1]g
    %[2]g

data "google_dns_managed_zone" "qa" {
  name = google_dns_managed_zone.foo.name
}
`,
		},

		{
			name: "assigned_array",
			block: `resource "google_compute_target_pool" "foo" {
	description = "Resource created for Terraform acceptance testing"
	name = "tpool-test-%s"
 	instances = [%s]
}
`,
			expected: `resource "google_compute_target_pool" "foo" {
  description = "Resource created for Terraform acceptance testing"
  name        = "tpool-test-%s"
  instances   = [%s]
}
`,
		},

		{
			name: "assigned",
			block: `resource "aws_alb_target_group" "test" {
  name = "%s"
  port = 443
  protocol = "HTTPS"
  vpc_id = "${aws_vpc.test.id}"

  deregistration_delay = 200

  stickiness {
    type = "lb_cookie"
    cookie_duration = 10000
  }

  health_check {
    path = "/health"
    interval = 60
    port = 8081
    protocol = "HTTP"
    timeout = 3
    healthy_threshold = 3
    unhealthy_threshold = 3
    matcher = "200-299"
  }

  tags = {
    TestName = "TestAccAWSALBTargetGroup_basic"
  }
}

resource "aws_vpc" "test" {
  cidr_block = "10.0.0.0/16"

  tags = {
    Name = "terraform-testacc-alb-target-group-basic"
  }
}`,
			expected: `resource "aws_alb_target_group" "test" {
  name     = "%s"
  port     = 443
  protocol = "HTTPS"
  vpc_id   = aws_vpc.test.id

  deregistration_delay = 200

  stickiness {
    type            = "lb_cookie"
    cookie_duration = 10000
  }

  health_check {
    path                = "/health"
    interval            = 60
    port                = 8081
    protocol            = "HTTP"
    timeout             = 3
    healthy_threshold   = 3
    unhealthy_threshold = 3
    matcher             = "200-299"
  }

  tags = {
    TestName = "TestAccAWSALBTargetGroup_basic"
  }
}

resource "aws_vpc" "test" {
  cidr_block = "10.0.0.0/16"

  tags = {
    Name = "terraform-testacc-alb-target-group-basic"
  }
}
`,
		},

		{
			name: "assigned-positional",
			block: `resource "google_compute_network" "ig_network" {
		name = "%[1]s"
		auto_create_subnetworks = true
	}
`,
			expected: `resource "google_compute_network" "ig_network" {
  name                    = "%[1]s"
  auto_create_subnetworks = true
}
`,
		},
	}

	t.Parallel()

	ctx := context.Background()

	tfBin, err := InstallTerraform(ctx)
	if err != nil {
		t.Fatalf("Failed to configure Terraform executable: %s", err)
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var errB strings.Builder
			log := common.CreateLogger(&errB)
			result, err := Upgrade12VerbBlock(ctx, tfBin, log, test.block)
			if err != nil && !test.error {
				t.Fatalf("Got an error when none was expected: %s", err)
			}
			if err == nil && test.error {
				t.Errorf("Expected an error and none was generated")
			}
			if result != test.expected {
				t.Errorf("Got: \n%#v\nexpected:\n%#v\n", result, test.expected)
			}
		})
	}
}
