# Has Diffs

```hcl
resource "aws_s3_bucket" "extra-lines" {
  
  bucket = "tf-test-bucket-extra-lines"
}
```

```hcl
resource "aws_s3_bucket" "no-errors" {
  bucket = "tf-test-bucket-no-errors"
}
```

```hcl
resource "aws_s3_bucket" "extra-space" {
  bucket    = "tf-test-bucket-extra-space"
}
```

```hcl
resource "aws_s3_bucket" "end-line" {
  bucket = "tf-test-bucket-end-line"
}
  
```

```hcl
     resource "aws_s3_bucket" "leading-space" {
  bucket = "tf-test-bucket-leading-space"
}
```
