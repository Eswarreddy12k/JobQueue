#!/usr/bin/env bash
#
# One-time AWS infrastructure setup for mini-job-queue.
# Prerequisites: aws cli, eksctl, kubectl
#
# What this script creates:
#   1. ECR repository (container image registry)
#   2. EKS cluster with 2 nodes (~$4.30/day — remember to teardown!)
#   3. IAM OIDC provider so GitHub Actions can authenticate without static keys
#   4. IAM role that GitHub Actions assumes to push images and deploy
#
# Usage:
#   export GITHUB_USER=<your-github-username>
#   chmod +x scripts/setup-aws.sh
#   ./scripts/setup-aws.sh
#
set -euo pipefail

# ─── Configuration ───────────────────────────────────────────────────────────
APP_NAME="mini-job-queue"
AWS_REGION="us-east-1"
EKS_CLUSTER="mini-job-queue"
NODE_TYPE="t3.medium"
NODE_COUNT=2
ROLE_NAME="github-actions-mini-job-queue"

# Your GitHub username — used in the OIDC trust policy
if [ -z "${GITHUB_USER:-}" ]; then
    echo "ERROR: Set GITHUB_USER first:  export GITHUB_USER=your-username"
    exit 1
fi

ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
echo "AWS Account: $ACCOUNT_ID"
echo "Region:      $AWS_REGION"
echo "GitHub User: $GITHUB_USER"
echo ""

# ─── Step 1: Create ECR Repository ──────────────────────────────────────────
echo "=== Step 1/6: Creating ECR repository ==="
aws ecr create-repository \
    --repository-name "$APP_NAME" \
    --region "$AWS_REGION" \
    --image-scanning-configuration scanOnPush=true \
    2>/dev/null || echo "  (already exists)"
echo "  ECR: $ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/$APP_NAME"
echo ""

# ─── Step 2: Create EKS Cluster ─────────────────────────────────────────────
echo "=== Step 2/6: Creating EKS cluster (this takes ~15 minutes) ==="
eksctl create cluster \
    --name "$EKS_CLUSTER" \
    --region "$AWS_REGION" \
    --node-type "$NODE_TYPE" \
    --nodes "$NODE_COUNT" \
    --nodes-min 1 \
    --nodes-max 3
# eksctl automatically updates ~/.kube/config
echo "  Cluster ready. kubectl is configured."
echo ""

# ─── Step 3: Create OIDC Provider for GitHub Actions ────────────────────────
# This lets GitHub Actions exchange a short-lived GitHub token for temporary
# AWS credentials — no long-lived access keys needed.
echo "=== Step 3/6: Creating GitHub OIDC provider ==="
aws iam create-open-id-connect-provider \
    --url "https://token.actions.githubusercontent.com" \
    --client-id-list "sts.amazonaws.com" \
    --thumbprint-list "6938fd4d98bab03faadb97b34396831e3780aea1" \
    2>/dev/null || echo "  (already exists)"
echo ""

# ─── Step 4: Create IAM Role for GitHub Actions ─────────────────────────────
# The trust policy says: "only GitHub Actions running from THIS repo can assume this role"
echo "=== Step 4/6: Creating IAM role ==="

OIDC_ARN="arn:aws:iam::${ACCOUNT_ID}:oidc-provider/token.actions.githubusercontent.com"

cat > /tmp/trust-policy.json <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Federated": "$OIDC_ARN"
            },
            "Action": "sts:AssumeRoleWithWebIdentity",
            "Condition": {
                "StringEquals": {
                    "token.actions.githubusercontent.com:aud": "sts.amazonaws.com"
                },
                "StringLike": {
                    "token.actions.githubusercontent.com:sub": "repo:${GITHUB_USER}/${APP_NAME}:*"
                }
            }
        }
    ]
}
EOF

aws iam create-role \
    --role-name "$ROLE_NAME" \
    --assume-role-policy-document file:///tmp/trust-policy.json \
    2>/dev/null || echo "  (role already exists)"

# Attach ECR push permission
aws iam attach-role-policy \
    --role-name "$ROLE_NAME" \
    --policy-arn "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryPowerUser"

echo "  Role ARN: arn:aws:iam::${ACCOUNT_ID}:role/${ROLE_NAME}"
echo ""

# ─── Step 5: Grant the Role Access to EKS ───────────────────────────────────
echo "=== Step 5/6: Granting EKS access to GitHub Actions role ==="
ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${ROLE_NAME}"

aws eks create-access-entry \
    --cluster-name "$EKS_CLUSTER" \
    --principal-arn "$ROLE_ARN" \
    --type STANDARD \
    2>/dev/null || echo "  (access entry already exists)"

aws eks associate-access-policy \
    --cluster-name "$EKS_CLUSTER" \
    --principal-arn "$ROLE_ARN" \
    --policy-arn "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy" \
    --access-scope type=cluster \
    2>/dev/null || echo "  (policy already associated)"
echo ""

# ─── Step 6: Initial Deploy ─────────────────────────────────────────────────
echo "=== Step 6/6: Deploying K8s manifests ==="
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/
echo ""

# ─── Done ────────────────────────────────────────────────────────────────────
echo "============================================="
echo "Setup complete!"
echo ""
echo "Next steps:"
echo "  1. Add this GitHub repository secret:"
echo "     Name:  AWS_ACCOUNT_ID"
echo "     Value: $ACCOUNT_ID"
echo ""
echo "  2. Push to main to trigger the CI/CD pipeline:"
echo "     git push origin main"
echo ""
echo "  3. IMPORTANT: Teardown when done to avoid charges (~\$4.30/day):"
echo "     ./scripts/teardown-aws.sh"
echo "============================================="
