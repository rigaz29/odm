// Package version holds the ODM release identity.
package version

// Version is the semantic version of this build. Release builds overwrite it
// with -ldflags "-X odm/internal/version.Version=<tag>".
var Version = "0.1.0"

// Name is the product name.
const Name = "Ochi Download Manager"

// Repo is the GitHub "owner/name" whose releases the update check reads.
// Forks overwrite it with -ldflags "-X odm/internal/version.Repo=<owner/name>".
var Repo = "rigaz29/odm"
