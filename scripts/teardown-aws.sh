#!/usr/bin/env bash
#
# Tears down all AWS resources created by setup-aws.sh.
# Run this when you're done to stop charges (~$4.30/day).
#
set -euo pipefail

APP_NAME="mini-job-queue"
AWS_REGION="us-east-1"
EKS_CLUSTER="mini-job-queue"
ROLE_NAME="github-actions-mini-job-queue"
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

echo "Tearing down AWS resources for $APP_NAME..."
echo ""

# ─── Step 1: Delete EKS Cluster (takes ~10 minutes) ─────────────────────────
echo "=== Deleting EKS cluster (this takes ~10 minutes) ==="
eksctl delete cluster --name "$EKS_CLUSTER" --region "$AWS_REGION" || true
echo ""

# ─── Step 2: Delete ECR Repository ──────────────────────────────────────────
echo "=== Deleting ECR repository ==="
aws ecr delete-repository \
    --repository-name "$APP_NAME" \
    --region "$AWS_REGION" \
    --force || true
echo ""

# ─── Step 3: Detach Policies and Delete IAM Role ────────────────────────────
echo "=== Cleaning up IAM role ==="
aws iam detach-role-policy \
    --role-name "$ROLE_NAME" \
    --policy-arn "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryPowerUser" || true

aws iam delete-role \
    --role-name "$ROLE_NAME" || true
echo ""

# ─── Step 4: Delete OIDC Provider ───────────────────────────────────────────
echo "=== Deleting GitHub OIDC provider ==="
OIDC_ARN="arn:aws:iam::${ACCOUNT_ID}:oidc-provider/token.actions.githubusercontent.com"
aws iam delete-open-id-connect-provider \
    --open-id-connect-provider-arn "$OIDC_ARN" || true
echo ""

echo "============================================="
echo "Teardown complete. All AWS resources removed."
echo "============================================="
