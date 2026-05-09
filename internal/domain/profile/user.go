package profile

// User mirrors the internal-service contract: ID, Name, License (a.k.a. Tier), Role.
//
// The assignment refers to the License field as "Tier" in the /profile output description;
// we keep the internal-service field name (License) and treat them as synonyms in docs.
type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"name"`
	License     string `json:"license"`
	Role        string `json:"role"`
}
