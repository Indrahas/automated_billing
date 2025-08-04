
# Terraform Skeleton (AWS)

This folder provides placeholders to provision:
- **MSK** (Kafka)
- **RDS Postgres**
- **S3** bucket for exports

> Fill in your VPC, subnets, and security groups. Consider using AWS Secrets Manager for credentials.

Suggested modules (add your own `main.tf`):
- `terraform-aws-modules/vpc/aws`
- `terraform-aws-modules/rds/aws`
- `terraform-aws-modules/msk-kafka-cluster/aws`
- `terraform-aws-modules/s3-bucket/aws`
