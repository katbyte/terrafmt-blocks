package test1

import (
	"fmt"
)

func testReturnSprintfSimple() string {
	return fmt.Sprintf(`
resource "aws_s3_bucket" "simple" {
  bucket = "tf-test-bucket-simple"
}
`)
}

func testReturnStringSimple() string {
	return `
resource "aws_s3_bucket" "simple2" {
  bucket = "tf-test-bucket-simple2"
}
`
}

func testReturnSprintfWithParameters(randInt int) string {
	return fmt.Sprintf(`
resource "aws_s3_bucket" "with-parameters" {
  bucket = "tf-test-bucket-with-parameters-%d"
}
`, randInt)
}

func testReturnSprintfWithParametersAndStringAppend(randInt int) string {
	return fmt.Sprintf(`
resource "aws_s3_bucket" "with-parameters-and-append" {
  bucket = "tf-test-bucket-parameters-and-append-%d"
}
`, randInt) + testReturnSprintfSimple()
}

const testConst = `
resource "aws_s3_bucket" "const" {
  bucket = "tf-test-bucket-const"
}
`

func testComposed(randInt int) string {
	return testReturnSprintfWithParameters(randInt) + fmt.Sprintf(`
resource "aws_s3_bucket" "composed" {
  bucket = "tf-test-bucket-composed-%d"
}
`, randInt)
}

func testDataSource() string {
	return fmt.Sprintf(`
data "aws_s3_bucket" "simple" {
  bucket = "tf-test-bucket-simple"
}
`)
}

func testLeadingWhiteSpace(randInt int) string {
	return fmt.Sprintf(`
    resource "aws_s3_bucket" "leading-space" {
  bucket = "tf-test-bucket-leading-space-%d"
}
`, randInt)
}

func testLeadingWhiteSpaceAndLine(randInt int) string {
	return fmt.Sprintf(`
    
    resource "aws_s3_bucket" "leading-space-and-line" {
  bucket = "tf-test-bucket-leading-space-and-line-%d"
}
`, randInt)
}

func notTerraformSimpleString() string {
	fmt.Sprintf("%d: bad create: \n%#v\n%#v", i, cm, tc.Create)
}

func notTerraformXML() string {
	return `<DescribeAccountAttributesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
	<requestId>7a62c49f-347e-4fc4-9331-6e8eEXAMPLE</requestId>
	<accountAttributeSet>
	  <item>
		<attributeName>supported-platforms</attributeName>
		<attributeValueSet>
		  <item>
			<attributeValue>VPC</attributeValue>
		  </item>
		  <item>
			<attributeValue>EC2</attributeValue>
		  </item>
		</attributeValueSet>
	  </item>
	</accountAttributeSet>
  </DescribeAccountAttributesResponse>`
}
