package test2

import (
	"fmt"
)

func testExtraLines() string {
	return fmt.Sprintf(`
resource "aws_s3_bucket" "extra-lines" {
  
  bucket = "tf-test-bucket-extra-lines"
}
`)
}

// This is included to verify blocks with diffs and no diffs in the same file
func testNoFormattingErrors(randInt int) string {
	return fmt.Sprintf(`
resource "aws_s3_bucket" "no-errors" {
  bucket = "tf-test-bucket-no-errors-%d"
}
`, randInt)
}

func testExtraSpace(randInt int) string {
	return fmt.Sprintf(`
resource "aws_s3_bucket" "extra-space" {
  bucket    = "tf-test-bucket-extra-space-%d"
}
`, randInt) + testReturnSprintfSimple()
}

func testFinishLineWhiteSpace(randInt int) string {
	return fmt.Sprintf(`
resource "aws_s3_bucket" "end-line" {
  bucket = "tf-test-bucket-end-line-%d"
}
  `, randInt)
}

func testNoPadding(randInt int) string {
	return fmt.Sprintf(`resource "aws_alb_target_group" "test" {
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
}`, targetGroupName)
}

func testLeadingWhiteSpace(randInt int) string {
	return fmt.Sprintf(`
    resource "aws_s3_bucket" "leading-space" {
  bucket = "tf-test-bucket-leading-space-%d"
}
`, randInt)
}
