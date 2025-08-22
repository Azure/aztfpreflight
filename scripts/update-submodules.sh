#!/usr/bin/env bash

#
# update-submodules.sh — automate intercept branches across submodules
#
# Overview
# - Input: a provider tag (e.g., v4.38.0) for terraform-provider-azurerm.
# - Phase 1 (terraform-provider-azurerm):
#   * cd submodules/terraform-provider-azurerm
#   * Fetch from `upstream` (if present) and `origin`
#   * Record current HEAD commit (to be cherry-picked)
#   * Checkout tags/<providerTag>
#   * Create/reset branch <providerTag>-intercept from the tag (git checkout -B)
#   * Cherry-pick the recorded commit:
#       - On conflicts: abort the cherry-pick and exit non-zero
#       - If empty (already applied): skip and continue
#   * Push the branch to origin with --force-with-lease
# - Phase 2 (go-azure-sdk):
#   * Read go.mod in terraform-provider-azurerm to detect required go-azure-sdk line
#     - Determine component: sdk or resource-manager, and the version (vX.Y.Z...)
#     - Build full tag: <component>/<version> (e.g., sdk/v0.20250728.1144148)
#   * cd submodules/go-azure-sdk
#   * Fetch from `upstream` (if present) and `origin`
#   * Record current HEAD commit
#   * Checkout tags/<component>/<version>
#   * Create/reset branch <component>/<version>-intercept (git checkout -B)
#   * Cherry-pick the recorded commit with the same conflict/empty handling
#   * Push to origin with --force-with-lease
# - Phase 3 (go-autorest):
#   * Read go.mod in terraform-provider-azurerm to detect go-autorest submodule
#     - Prefer autorest, fallback to tracing, or any go-autorest/<component>
#     - Build full tag: <component>/<version> (e.g., autorest/v0.11.30)
#   * cd submodules/go-autorest and repeat the same steps as above
# - Finalize:
#   * cd repo root
#   * Run `go mod tidy && go mod vendor` to normalize module state and vendor deps
#
# Idempotency & safety
# - Uses `git checkout -B` to reset or create local intercept branches from the tag
# - Uses `git push --force-with-lease` to safely overwrite remote intercept branches
# - Requires a clean working tree in each submodule (no uncommitted changes)
# - If `upstream` is not configured, the script skips it and still uses `origin`
#
# Requirements
# - git installed, network access to remotes, and submodules initialized under:
#   * submodules/terraform-provider-azurerm
#   * submodules/go-azure-sdk
#   * submodules/go-autorest
#
# Example
#   scripts/update-submodules.sh v4.38.0
#

# Exit on error, undefined var, and errors in pipelines
set -euo pipefail

usage() {
  cat <<EOF
Usage: $(basename "$0") <tag>

Example:
  $(basename "$0") v4.38.0

Steps performed:
  1) Go to submodules/terraform-provider-azurerm
  2) Record current HEAD commit
  3) Checkout the tag (tags/<tag>)
  4) Create branch <tag>-intercept
  5) Cherry-pick previously recorded commit (abort on conflicts)
  6) Push the new branch to origin
EOF
}

log() { printf "[%s] %s\n" "$(date '+%Y-%m-%d %H:%M:%S')" "$*"; }
err() { printf "ERROR: %s\n" "$*" >&2; }

TAG="${1:-}"
if [[ -z "${TAG}" ]]; then
  usage
  exit 1
fi

# Resolve paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
SUBMODULE_DIR="${REPO_ROOT}/submodules/terraform-provider-azurerm"

if [[ ! -d "${SUBMODULE_DIR}" ]]; then
  err "Submodule directory not found: ${SUBMODULE_DIR}"
  exit 1
fi

pushd "${SUBMODULE_DIR}" >/dev/null

# Ensure this is a git repository
if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  err "Not a git repository: ${SUBMODULE_DIR}"
  popd >/dev/null || true
  exit 1
fi

# Ensure clean working tree
if [[ -n "$(git status --porcelain)" ]]; then
  err "Working tree is dirty in ${SUBMODULE_DIR}. Commit or stash changes first."
  popd >/dev/null || true
  exit 1
fi

UPSTREAM_EXISTS=false
if git remote get-url upstream >/dev/null 2>&1; then
  UPSTREAM_EXISTS=true
  log "Fetching from upstream (tags and branches)…"
  git fetch upstream --tags --prune >/dev/null
else
  log "No 'upstream' remote found; skipping upstream fetch."
fi

log "Fetching from origin (tags and branches)…"
git fetch origin --tags --prune >/dev/null

# Record current HEAD commit
LAST_COMMIT_SHA="$(git rev-parse HEAD)"
LAST_COMMIT_ONELINE="$(git log -1 --pretty=format:'%h %s')"
log "Recorded current HEAD commit: ${LAST_COMMIT_ONELINE}"

# Verify the tag exists (if not, try another upstream fetch just in case)
if ! git rev-parse -q --verify "refs/tags/${TAG}" >/dev/null; then
  if [[ "${UPSTREAM_EXISTS}" == "true" ]]; then
    log "Tag ${TAG} not found locally; refetching tags from upstream…"
    git fetch upstream --tags --prune >/dev/null || true
  fi

  if ! git rev-parse -q --verify "refs/tags/${TAG}" >/dev/null; then
    err "Tag not found: ${TAG}"
    popd >/dev/null || true
    exit 1
  fi
fi

log "Checking out tag: tags/${TAG} (detached HEAD)…"
git checkout -q "tags/${TAG}"

# Create new branch from the tag
INTERCEPT_BRANCH="${TAG}-intercept"

log "Creating/resetting branch: ${INTERCEPT_BRANCH} from tag ${TAG}"
git checkout -q -B "${INTERCEPT_BRANCH}"

# Cherry-pick the previously recorded commit
log "Cherry-picking commit ${LAST_COMMIT_SHA} onto ${INTERCEPT_BRANCH}…"
set +e
git cherry-pick -x "${LAST_COMMIT_SHA}"
CHERRYPICK_STATUS=$?
set -e

if [[ ${CHERRYPICK_STATUS} -ne 0 ]]; then
  # Check if there are merge conflicts
  if git diff --name-only --diff-filter=U | grep -q .; then
    err "Cherry-pick has conflicts. Aborting cherry-pick and exiting."
    git cherry-pick --abort >/dev/null 2>&1 || true
    popd >/dev/null || true
    exit ${CHERRYPICK_STATUS}
  else
    # No conflicts: likely an empty cherry-pick (commit already included). Skip and continue.
    log "Cherry-pick produced no changes (already applied). Skipping."
    git cherry-pick --skip >/dev/null 2>&1 || true
  fi
fi

log "Pushing branch ${INTERCEPT_BRANCH} to origin (force-with-lease)…"
git push -u origin "${INTERCEPT_BRANCH}" --force-with-lease

popd >/dev/null
log "Done. Created and pushed branch ${INTERCEPT_BRANCH} based on tag ${TAG} with cherry-picked commit ${LAST_COMMIT_SHA}."

# ------------------------------------------------------------
# Second phase: sync go-azure-sdk submodule with matching tag
# ------------------------------------------------------------

# Determine the go-azure-sdk version required by the azurerm submodule
AZURERM_GO_MOD="${SUBMODULE_DIR}/go.mod"
if [[ ! -f "${AZURERM_GO_MOD}" ]]; then
  err "go.mod not found in ${SUBMODULE_DIR}; cannot determine go-azure-sdk version."
  exit 1
fi

log "Reading go-azure-sdk version from ${AZURERM_GO_MOD}…"

# Extract component (sdk or resource-manager) and version from require lines
read -r GO_AZURE_SDK_COMPONENT GO_AZURE_SDK_TAG < <(awk '
  $1 == "github.com/hashicorp/go-azure-sdk/sdk" { print "sdk", $2; exit }
  $1 == "github.com/hashicorp/go-azure-sdk/resource-manager" { print "resource-manager", $2; exit }
  # handle indentation inside require ( ... )
  $2 == "github.com/hashicorp/go-azure-sdk/sdk" { print "sdk", $3; exit }
  $2 == "github.com/hashicorp/go-azure-sdk/resource-manager" { print "resource-manager", $3; exit }
' "${AZURERM_GO_MOD}")

# Fallback: handle bare path (rare) — default to sdk component
if [[ -z "${GO_AZURE_SDK_TAG:-}" ]]; then
  read -r GO_AZURE_SDK_COMPONENT GO_AZURE_SDK_TAG < <(awk '
    $1 == "github.com/hashicorp/go-azure-sdk" { print "sdk", $2; exit }
    $2 == "github.com/hashicorp/go-azure-sdk" { print "sdk", $3; exit }
  ' "${AZURERM_GO_MOD}") || true
fi

if [[ -z "${GO_AZURE_SDK_TAG:-}" || -z "${GO_AZURE_SDK_COMPONENT:-}" ]]; then
  err "Could not find github.com/hashicorp/go-azure-sdk version in ${AZURERM_GO_MOD}."
  exit 1
fi

GO_AZURE_SDK_FULL_TAG="${GO_AZURE_SDK_COMPONENT}/${GO_AZURE_SDK_TAG}"
log "Detected github.com/hashicorp/go-azure-sdk tag: ${GO_AZURE_SDK_FULL_TAG}"

GO_AZURE_SDK_DIR="${REPO_ROOT}/submodules/go-azure-sdk"
if [[ ! -d "${GO_AZURE_SDK_DIR}" ]]; then
  err "Submodule directory not found: ${GO_AZURE_SDK_DIR}"
  exit 1
fi

pushd "${GO_AZURE_SDK_DIR}" >/dev/null

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  err "Not a git repository: ${GO_AZURE_SDK_DIR}"
  popd >/dev/null || true
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  err "Working tree is dirty in ${GO_AZURE_SDK_DIR}. Commit or stash changes first."
  popd >/dev/null || true
  exit 1
fi

UPSTREAM_EXISTS=false
if git remote get-url upstream >/dev/null 2>&1; then
  UPSTREAM_EXISTS=true
  log "Fetching from upstream (go-azure-sdk)…"
  git fetch upstream --tags --prune >/dev/null
else
  log "No 'upstream' remote found in go-azure-sdk; skipping upstream fetch."
fi

log "Fetching from origin (go-azure-sdk)…"
git fetch origin --tags --prune >/dev/null

# Record current commit in go-azure-sdk
SDK_LAST_COMMIT_SHA="$(git rev-parse HEAD)"
SDK_LAST_COMMIT_ONELINE="$(git log -1 --pretty=format:'%h %s')"
log "go-azure-sdk current HEAD: ${SDK_LAST_COMMIT_ONELINE}"

# Verify tag exists
if ! git rev-parse -q --verify "refs/tags/${GO_AZURE_SDK_FULL_TAG}" >/dev/null; then
  if [[ "${UPSTREAM_EXISTS}" == "true" ]]; then
    log "Tag ${GO_AZURE_SDK_FULL_TAG} not found locally; refetching from upstream…"
    git fetch upstream --tags --prune >/dev/null || true
  fi
fi

if ! git rev-parse -q --verify "refs/tags/${GO_AZURE_SDK_FULL_TAG}" >/dev/null; then
  err "Tag not found in go-azure-sdk: ${GO_AZURE_SDK_FULL_TAG}"
  popd >/dev/null || true
  exit 1
fi

log "Checking out go-azure-sdk tag: tags/${GO_AZURE_SDK_FULL_TAG} (detached HEAD)…"
git checkout -q "tags/${GO_AZURE_SDK_FULL_TAG}"

SDK_INTERCEPT_BRANCH="${GO_AZURE_SDK_FULL_TAG}-intercept"

log "Creating/resetting go-azure-sdk branch: ${SDK_INTERCEPT_BRANCH} from tag ${GO_AZURE_SDK_FULL_TAG}"
git checkout -q -B "${SDK_INTERCEPT_BRANCH}"

log "Cherry-picking commit ${SDK_LAST_COMMIT_SHA} onto ${SDK_INTERCEPT_BRANCH}…"
set +e
git cherry-pick -x "${SDK_LAST_COMMIT_SHA}"
SDK_CHERRYPICK_STATUS=$?
set -e

if [[ ${SDK_CHERRYPICK_STATUS} -ne 0 ]]; then
  if git diff --name-only --diff-filter=U | grep -q .; then
    err "Cherry-pick has conflicts in go-azure-sdk. Aborting and exiting."
    git cherry-pick --abort >/dev/null 2>&1 || true
    popd >/dev/null || true
    exit ${SDK_CHERRYPICK_STATUS}
  else
    log "Cherry-pick produced no changes in go-azure-sdk (already applied). Skipping."
    git cherry-pick --skip >/dev/null 2>&1 || true
  fi
fi

log "Pushing go-azure-sdk branch ${SDK_INTERCEPT_BRANCH} to origin (force-with-lease)…"
git push -u origin "${SDK_INTERCEPT_BRANCH}" --force-with-lease

popd >/dev/null
log "Done. Created and pushed go-azure-sdk branch ${SDK_INTERCEPT_BRANCH} based on tag ${GO_AZURE_SDK_FULL_TAG} with cherry-picked commit ${SDK_LAST_COMMIT_SHA}."

# ------------------------------------------------------------
# Third phase: sync go-autorest submodule with matching tag
# ------------------------------------------------------------

log "Reading go-autorest version from ${AZURERM_GO_MOD}…"

# Try preferred module 'autorest'
read -r GO_AUTOREST_COMPONENT GO_AUTOREST_TAG < <(awk '
  $1 == "github.com/Azure/go-autorest/autorest" { print "autorest", $2; exit }
  $2 == "github.com/Azure/go-autorest/autorest" { print "autorest", $3; exit }
' "${AZURERM_GO_MOD}") || true

# Fallback to 'tracing'
if [[ -z "${GO_AUTOREST_TAG:-}" ]]; then
  read -r GO_AUTOREST_COMPONENT GO_AUTOREST_TAG < <(awk '
    $1 == "github.com/Azure/go-autorest/tracing" { print "tracing", $2; exit }
    $2 == "github.com/Azure/go-autorest/tracing" { print "tracing", $3; exit }
  ' "${AZURERM_GO_MOD}") || true
fi

# Fallback to any go-autorest submodule (skip base path without subpath)
if [[ -z "${GO_AUTOREST_TAG:-}" ]]; then
  read -r GO_AUTOREST_COMPONENT GO_AUTOREST_TAG < <(awk '
    $1 ~ /^github.com\/Azure\/go-autorest\// { path=$1; ver=$2; sub(/^github.com\/Azure\/go-autorest\//, "", path); if (path != "") { print path, ver; exit } }
    $2 ~ /^github.com\/Azure\/go-autorest\// { path=$2; ver=$3; sub(/^github.com\/Azure\/go-autorest\//, "", path); if (path != "") { print path, ver; exit } }
  ' "${AZURERM_GO_MOD}") || true
fi

if [[ -z "${GO_AUTOREST_TAG:-}" || -z "${GO_AUTOREST_COMPONENT:-}" ]]; then
  err "Could not find github.com/Azure/go-autorest version in ${AZURERM_GO_MOD}."
  exit 1
fi

GO_AUTOREST_FULL_TAG="${GO_AUTOREST_COMPONENT}/${GO_AUTOREST_TAG}"
log "Detected github.com/Azure/go-autorest tag: ${GO_AUTOREST_FULL_TAG}"

GO_AUTOREST_DIR="${REPO_ROOT}/submodules/go-autorest"
if [[ ! -d "${GO_AUTOREST_DIR}" ]]; then
  err "Submodule directory not found: ${GO_AUTOREST_DIR}"
  exit 1
fi

pushd "${GO_AUTOREST_DIR}" >/dev/null

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  err "Not a git repository: ${GO_AUTOREST_DIR}"
  popd >/dev/null || true
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  err "Working tree is dirty in ${GO_AUTOREST_DIR}. Commit or stash changes first."
  popd >/dev/null || true
  exit 1
fi

UPSTREAM_EXISTS=false
if git remote get-url upstream >/dev/null 2>&1; then
  UPSTREAM_EXISTS=true
  log "Fetching from upstream (go-autorest)…"
  git fetch upstream --tags --prune >/dev/null
else
  log "No 'upstream' remote found in go-autorest; skipping upstream fetch."
fi

log "Fetching from origin (go-autorest)…"
git fetch origin --tags --prune >/dev/null

# Record current commit in go-autorest
AUTO_LAST_COMMIT_SHA="$(git rev-parse HEAD)"
AUTO_LAST_COMMIT_ONELINE="$(git log -1 --pretty=format:'%h %s')"
log "go-autorest current HEAD: ${AUTO_LAST_COMMIT_ONELINE}"

# Verify tag exists
if ! git rev-parse -q --verify "refs/tags/${GO_AUTOREST_FULL_TAG}" >/dev/null; then
  if [[ "${UPSTREAM_EXISTS}" == "true" ]]; then
    log "Tag ${GO_AUTOREST_FULL_TAG} not found locally; refetching from upstream…"
    git fetch upstream --tags --prune >/dev/null || true
  fi
fi

if ! git rev-parse -q --verify "refs/tags/${GO_AUTOREST_FULL_TAG}" >/dev/null; then
  err "Tag not found in go-autorest: ${GO_AUTOREST_FULL_TAG}"
  popd >/dev/null || true
  exit 1
fi

log "Checking out go-autorest tag: tags/${GO_AUTOREST_FULL_TAG} (detached HEAD)…"
git checkout -q "tags/${GO_AUTOREST_FULL_TAG}"

AUTO_INTERCEPT_BRANCH="${GO_AUTOREST_FULL_TAG}-intercept"

log "Creating/resetting go-autorest branch: ${AUTO_INTERCEPT_BRANCH} from tag ${GO_AUTOREST_FULL_TAG}"
git checkout -q -B "${AUTO_INTERCEPT_BRANCH}"

log "Cherry-picking commit ${AUTO_LAST_COMMIT_SHA} onto ${AUTO_INTERCEPT_BRANCH}…"
set +e
git cherry-pick -x "${AUTO_LAST_COMMIT_SHA}"
AUTO_CHERRYPICK_STATUS=$?
set -e

if [[ ${AUTO_CHERRYPICK_STATUS} -ne 0 ]]; then
  if git diff --name-only --diff-filter=U | grep -q .; then
    err "Cherry-pick has conflicts in go-autorest. Aborting and exiting."
    git cherry-pick --abort >/dev/null 2>&1 || true
    popd >/dev/null || true
    exit ${AUTO_CHERRYPICK_STATUS}
  else
    log "Cherry-pick produced no changes in go-autorest (already applied). Skipping."
    git cherry-pick --skip >/dev/null 2>&1 || true
  fi
fi

log "Pushing go-autorest branch ${AUTO_INTERCEPT_BRANCH} to origin (force-with-lease)…"
git push -u origin "${AUTO_INTERCEPT_BRANCH}" --force-with-lease

popd >/dev/null
log "Done. Created and pushed go-autorest branch ${AUTO_INTERCEPT_BRANCH} based on tag ${GO_AUTOREST_FULL_TAG} with cherry-picked commit ${AUTO_LAST_COMMIT_SHA}."

# ------------------------------------------------------------
# Finalize: tidy and vendor in repo root
# ------------------------------------------------------------

if ! command -v go >/dev/null 2>&1; then
  err "Go toolchain not found in PATH; cannot run 'go mod tidy' and 'go mod vendor'."
  exit 1
fi

log "Tidying and vendoring dependencies in repo root…"
pushd "${REPO_ROOT}" >/dev/null
go mod tidy
go mod vendor
popd >/dev/null
log "Dependency tidy and vendoring completed in repo root."
