# Release Workflow Version Automation

**Date:** 20250407

## Request

Modify the GitHub release workflow to:
1. Set the release version in version.go (remove the '+src' suffix) before publishing
2. After publishing the release, bump to the next dev version (increase the minor version and add the '+src' suffix)

## Implementation

### Changes to `.github/workflows/go-release.yml`

Added four new steps to the workflow:

1. **Set release version** (pre-release step):
   - Extracts version from the pushed tag (handles both `v1.2.3` and `1.2.3` formats)
   - Stores version in `RELEASE_VERSION` environment variable
   - Uses `sed` to update `version.go`, replacing the version string and removing `+src`
   - Only runs on tag pushes (`startsWith(github.ref, 'refs/tags/')`)

2. **Commit release version** (pre-release step):
   - Configures git with GitHub Actions bot identity
   - Commits the version change with message: `chore: set version to {VERSION} for release`
   - Pushes to the repository

3. **Bump to next dev version** (post-release step):
   - Parses the release version into major, minor, patch components
   - Increments the minor version (e.g., 1.5.0 → 1.6.0)
   - Appends `+src` suffix to indicate development version
   - Uses `sed` to update `version.go` with the new dev version
   - Only runs if the release was successful (`success()`)

4. **Commit next dev version** (post-release step):
   - Commits the dev version change with message: `chore: bump to next dev version`
   - Pushes to the repository

### Example Flow

When pushing tag `v1.5.0`:
1. Workflow starts with `version.go` containing: `var Version = "1.5.0+src"`
2. **Pre-release**: Updates to `var Version = "1.5.0"` and commits
3. GoReleaser builds and publishes release with clean version
4. **Post-release**: Updates to `var Version = "1.6.0+src"` and commits

## Key Implementation Details

- **Token**: Added `token: ${{ secrets.GITHUB_TOKEN }}` to checkout step to enable push operations
- **Conditionals**: All version-related steps only run on tag pushes (not pull requests)
- **Version parsing**: Uses shell parameter expansion and `cut` for robust version component extraction
- **Success guard**: Post-release steps only execute if GoReleaser succeeded

## Technical Notes

- The `+src` suffix in Go module versioning is used to indicate builds from source (development builds) as opposed to official releases
- The workflow assumes semantic versioning (major.minor.patch format)
- Git commits are made with the standard GitHub Actions bot identity
