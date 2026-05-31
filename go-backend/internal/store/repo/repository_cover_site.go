package repo

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-backend/internal/store/model"
)

func (r *Repository) ListCoverDomainProfiles() ([]model.CoverDomainProfile, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("repository not initialized")
	}
	var rows []model.CoverDomainProfile
	err := r.db.Order("id ASC").Find(&rows).Error
	return rows, err
}

func (r *Repository) GetCoverDomainProfile(id int64) (*model.CoverDomainProfile, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("repository not initialized")
	}
	var row model.CoverDomainProfile
	err := r.db.Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}

func (r *Repository) GetCoverDomainProfileByName(name string) (*model.CoverDomainProfile, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("repository not initialized")
	}
	var row model.CoverDomainProfile
	err := r.db.Where("name = ?", name).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}

func (r *Repository) UpsertCoverDomainProfile(profile *model.CoverDomainProfile) error {
	if r == nil || r.db == nil {
		return errors.New("repository not initialized")
	}
	if profile == nil {
		return errors.New("invalid cover domain profile")
	}
	now := time.Now().UnixMilli()
	if profile.CreatedTime <= 0 {
		profile.CreatedTime = now
	}
	profile.UpdatedTime = now
	if profile.ID > 0 {
		return r.db.Model(&model.CoverDomainProfile{}).
			Where("id = ?", profile.ID).
			Updates(map[string]interface{}{
				"name":             profile.Name,
				"enabled":          profile.Enabled,
				"site_label":       profile.SiteLabel,
				"domains":          profile.Domains,
				"cert_profile":     profile.CertProfile,
				"dns_provider":     profile.DNSProvider,
				"dns_profile":      profile.DNSProfile,
				"template_profile": profile.TemplateProfile,
				"upstream_origin":  profile.UpstreamOrigin,
				"static_html":      profile.StaticHTML,
				"updated_time":     profile.UpdatedTime,
			}).Error
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"enabled",
			"site_label",
			"domains",
			"cert_profile",
			"dns_provider",
			"dns_profile",
			"template_profile",
			"upstream_origin",
			"static_html",
			"updated_time",
		}),
	}).Create(profile).Error
}

func (r *Repository) DeleteCoverDomainProfile(id int64) error {
	if r == nil || r.db == nil {
		return errors.New("repository not initialized")
	}
	if id <= 0 {
		return errors.New("invalid cover domain profile id")
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("profile_id = ?", id).Delete(&model.TunnelCoverBinding{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.CoverDomainProfile{}, id).Error
	})
}

func (r *Repository) ListTunnelCoverBindings() ([]model.TunnelCoverBinding, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("repository not initialized")
	}
	var rows []model.TunnelCoverBinding
	err := r.db.Order("tunnel_id ASC, profile_id ASC").Find(&rows).Error
	return rows, err
}

func (r *Repository) ListTunnelCoverBindingsByTunnelID(tunnelID int64) ([]model.TunnelCoverBinding, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("repository not initialized")
	}
	if tunnelID <= 0 {
		return nil, nil
	}
	var rows []model.TunnelCoverBinding
	err := r.db.Where("tunnel_id = ? AND enabled = 1", tunnelID).
		Order("profile_id ASC").
		Find(&rows).Error
	return rows, err
}

func (r *Repository) ListTunnelCoverBindingsByProfileID(profileID int64) ([]model.TunnelCoverBinding, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("repository not initialized")
	}
	if profileID <= 0 {
		return nil, nil
	}
	var rows []model.TunnelCoverBinding
	err := r.db.Where("profile_id = ? AND enabled = 1", profileID).
		Order("tunnel_id ASC").
		Find(&rows).Error
	return rows, err
}

func (r *Repository) ReplaceTunnelCoverBindings(tunnelID int64, profileIDs []int64) error {
	if r == nil || r.db == nil {
		return errors.New("repository not initialized")
	}
	if tunnelID <= 0 {
		return errors.New("invalid tunnel id")
	}
	now := time.Now().UnixMilli()
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tunnel_id = ?", tunnelID).Delete(&model.TunnelCoverBinding{}).Error; err != nil {
			return err
		}
		seen := make(map[int64]struct{}, len(profileIDs))
		for _, profileID := range profileIDs {
			if profileID <= 0 {
				continue
			}
			if _, ok := seen[profileID]; ok {
				continue
			}
			seen[profileID] = struct{}{}
			row := model.TunnelCoverBinding{
				TunnelID:    tunnelID,
				ProfileID:   profileID,
				Enabled:     1,
				CreatedTime: now,
				UpdatedTime: now,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) ListCoverDomainProfilesByTunnelID(tunnelID int64) ([]model.CoverDomainProfile, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("repository not initialized")
	}
	if tunnelID <= 0 {
		return nil, nil
	}
	var rows []model.CoverDomainProfile
	err := r.db.Model(&model.CoverDomainProfile{}).
		Joins("JOIN tunnel_cover_binding ON tunnel_cover_binding.profile_id = cover_domain_profile.id").
		Where("tunnel_cover_binding.enabled = 1 AND tunnel_cover_binding.tunnel_id = ?", tunnelID).
		Order("cover_domain_profile.id ASC").
		Find(&rows).Error
	return rows, err
}

func (r *Repository) ListEnabledCoverDomainProfilesByTunnelIDs(tunnelIDs []int64) ([]model.CoverDomainProfile, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("repository not initialized")
	}
	if len(tunnelIDs) == 0 {
		return nil, nil
	}
	var rows []model.CoverDomainProfile
	err := r.db.Model(&model.CoverDomainProfile{}).
		Joins("JOIN tunnel_cover_binding ON tunnel_cover_binding.profile_id = cover_domain_profile.id").
		Where("cover_domain_profile.enabled = 1 AND tunnel_cover_binding.enabled = 1 AND tunnel_cover_binding.tunnel_id IN ?", tunnelIDs).
		Order("cover_domain_profile.id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]model.CoverDomainProfile, 0, len(rows))
	seen := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		if row.ID <= 0 {
			continue
		}
		if _, ok := seen[row.ID]; ok {
			continue
		}
		seen[row.ID] = struct{}{}
		out = append(out, row)
	}
	return out, nil
}

// Legacy tunnel-level cover profiles are kept for rollback compatibility.
func (r *Repository) ListTunnelCoverProfiles() ([]model.TunnelCoverProfile, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("repository not initialized")
	}
	var rows []model.TunnelCoverProfile
	err := r.db.Order("tunnel_id ASC").Find(&rows).Error
	return rows, err
}

func (r *Repository) GetTunnelCoverProfile(tunnelID int64) (*model.TunnelCoverProfile, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("repository not initialized")
	}
	var row model.TunnelCoverProfile
	err := r.db.Where("tunnel_id = ?", tunnelID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}

func (r *Repository) UpsertTunnelCoverProfile(profile *model.TunnelCoverProfile) error {
	if r == nil || r.db == nil {
		return errors.New("repository not initialized")
	}
	if profile == nil || profile.TunnelID <= 0 {
		return errors.New("invalid tunnel cover profile")
	}
	now := time.Now().UnixMilli()
	if profile.CreatedTime <= 0 {
		profile.CreatedTime = now
	}
	profile.UpdatedTime = now
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tunnel_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"enabled",
			"site_label",
			"domains",
			"cert_profile",
			"dns_provider",
			"dns_profile",
			"template_profile",
			"upstream_origin",
			"static_html",
			"updated_time",
		}),
	}).Create(profile).Error
}

func (r *Repository) ListEnabledTunnelCoverProfilesByTunnelIDs(tunnelIDs []int64) ([]model.TunnelCoverProfile, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("repository not initialized")
	}
	if len(tunnelIDs) == 0 {
		return nil, nil
	}
	var rows []model.TunnelCoverProfile
	err := r.db.Where("enabled = 1 AND tunnel_id IN ?", tunnelIDs).
		Order("tunnel_id ASC").
		Find(&rows).Error
	return rows, err
}

func (r *Repository) ListNodeCoverServices() ([]model.NodeCoverService, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("repository not initialized")
	}
	var rows []model.NodeCoverService
	err := r.db.Order("node_id ASC").Find(&rows).Error
	return rows, err
}

func (r *Repository) GetNodeCoverService(nodeID int64) (*model.NodeCoverService, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("repository not initialized")
	}
	var row model.NodeCoverService
	err := r.db.Where("node_id = ?", nodeID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}

func (r *Repository) UpsertNodeCoverService(service *model.NodeCoverService) error {
	if r == nil || r.db == nil {
		return errors.New("repository not initialized")
	}
	if service == nil || service.NodeID <= 0 {
		return errors.New("invalid node cover service")
	}
	now := time.Now().UnixMilli()
	if service.CreatedTime <= 0 {
		service.CreatedTime = now
	}
	service.UpdatedTime = now
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "node_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"enabled",
			"public_port",
			"local_listen",
			"last_sync_time",
			"last_status",
			"updated_time",
		}),
	}).Create(service).Error
}

func (r *Repository) UpdateNodeCoverServiceStatus(nodeID int64, status string, syncTime int64) error {
	if r == nil || r.db == nil {
		return errors.New("repository not initialized")
	}
	if nodeID <= 0 {
		return errors.New("invalid node id")
	}
	if syncTime <= 0 {
		syncTime = time.Now().UnixMilli()
	}
	return r.db.Model(&model.NodeCoverService{}).
		Where("node_id = ?", nodeID).
		Updates(map[string]interface{}{
			"last_sync_time": syncTime,
			"last_status":    status,
			"updated_time":   syncTime,
		}).Error
}
