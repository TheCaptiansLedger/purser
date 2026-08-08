package stashdb

// GraphQL query documents this adapter issues, one unexported constant per
// operation, built from shared field-selection fragments so
// ports.Performer/Studio/Scene's mapped fields stay in one place. Field
// names below (birth_date, is_favorite, release_date, ...) are stash-box's
// own schema field names, confirmed against
// https://github.com/stashapp/stash-box (graphql/schema/types/*.graphql)
// rather than assumed.

// performerFields is the selection set behind ports.Performer.
const performerFields = `
	id
	name
	disambiguation
	aliases
	gender
	urls { url site { id name url } }
	birth_date
	ethnicity
	country
	eye_color
	hair_color
	height
	cup_size
	band_size
	waist_size
	hip_size
	breast_type
	career_start_year
	career_end_year
	images { id url width height }
	is_favorite
	deleted
	merged_ids
`

// studioFields is the selection set behind ports.Studio. Parent/
// child_studios are deliberately shallow (id/name only) — see
// ports.Studio's doc comment.
const studioFields = `
	id
	name
	urls { url site { id name url } }
	parent { id name }
	child_studios { id name }
	images { id url width height }
	deleted
	is_favorite
`

// sceneFields is the selection set behind ports.Scene.
const sceneFields = `
	id
	title
	details
	release_date
	urls { url site { id name url } }
	studio { ` + studioFields + ` }
	tags { id name description aliases }
	images { id url width height }
	performers { performer { ` + performerFields + ` } as }
	fingerprints { hash algorithm duration submissions user_submitted }
	duration
	director
	code
	deleted
`

const (
	queryFindPerformer = `query FindPerformer($id: ID!) { findPerformer(id: $id) { ` + performerFields + ` } }`

	querySearchPerformer = `query SearchPerformer($term: String!) { searchPerformer(term: $term) { ` + performerFields + ` } }`

	queryFindStudio = `query FindStudio($id: ID!) { findStudio(id: $id) { ` + studioFields + ` } }`

	queryFindScene = `query FindScene($id: ID!) { findScene(id: $id) { ` + sceneFields + ` } }`

	querySearchScene = `query SearchScene($term: String!) { searchScene(term: $term) { ` + sceneFields + ` } }`

	queryFindScenesBySceneFingerprints = `query FindScenesBySceneFingerprints($fingerprints: [[FingerprintQueryInput!]!]!) { findScenesBySceneFingerprints(fingerprints: $fingerprints) { ` + sceneFields + ` } }`
)
