package model

type GroupsCache struct {
	Groups        map[string]bool
	UsersInGroups map[string]map[string]bool
}

func (u *GroupsCache) AddUserToGroup(user string, groupName string) {
	if _, ok := u.UsersInGroups[groupName]; !ok {
		u.UsersInGroups[groupName] = make(map[string]bool)
	}
	u.UsersInGroups[groupName][user] = true
}

func (u *GroupsCache) RemoveUserFromGroup(user string, groupName string) {
	if _, ok := u.UsersInGroups[groupName]; !ok {
		u.UsersInGroups[groupName] = make(map[string]bool)
	}
	u.UsersInGroups[groupName][user] = false
}

func (u *GroupsCache) CreateGroup(groupName string, participants []string) {
	u.Groups[groupName] = true
	for _, user := range participants {
		u.AddUserToGroup(user, groupName)
	}
}

func (u *GroupsCache) DeleteGroup(groupName string) {
	u.Groups[groupName] = false
	delete(u.UsersInGroups, groupName)
}

type FunctionCachedUpdates struct {
	DirtyCollections   map[string]map[string]string
	CollectionsGroups  map[string]map[string]bool
	DeletedRecords     map[string][]string
	DeletedCollections []string
	Groups             GroupsCache
}

type WasmResponse struct {
	HttpCode     int                    `json:"http_code"`
	Response     any                    `json:"response"`
	CacheUpdates *FunctionCachedUpdates `json:"cache_updates,omitempty"`
}
