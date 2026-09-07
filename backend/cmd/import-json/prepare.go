package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type repair struct {
	Section string            `json:"section"`
	Index   int               `json:"index"`
	Reason  string            `json:"reason"`
	Changes map[string]string `json:"changes"`
}

func prepareSource(dir, destination string) error {
	s, hashes, err := loadSource(dir)
	if err != nil {
		return err
	}
	repairs, err := prepareRepairs(&s)
	if err != nil {
		return err
	}
	if err := validateSource(s); err != nil {
		return err
	}
	if err := os.Mkdir(destination, 0700); err != nil {
		return fmt.Errorf("output must be a new directory: %w", err)
	}
	for name := range hashes {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		if name == "files.dev.json" || name == "customers.dev.json" {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(raw, &fields); err != nil {
				return err
			}
			for _, section := range []string{"files", "documentRequests"} {
				if fields[section] == nil {
					continue
				}
				var rows []map[string]json.RawMessage
				if err := json.Unmarshal(fields[section], &rows); err != nil {
					return err
				}
				for _, change := range repairs {
					if change.Section == section {
						for key, value := range change.Changes {
							rows[change.Index][key], _ = json.Marshal(value)
						}
					}
				}
				fields[section], err = json.Marshal(rows)
				if err != nil {
					return err
				}
			}
			raw, err = json.MarshalIndent(fields, "", "  ")
			if err != nil {
				return err
			}
		}
		if err := os.WriteFile(filepath.Join(destination, name), raw, 0600); err != nil {
			return err
		}
	}
	report := map[string]any{"sourceSHA256": hashes, "repairs": repairs, "sourceUnchanged": true}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(destination, "preparation-report.json"), raw, 0600); err != nil {
		return err
	}
	fmt.Printf("Prepared separate snapshot with %d recorded repairs; source files were not changed.\n", len(repairs))
	return nil
}

func prepareRepairs(s *source) ([]repair, error) {
	var repairs []repair
	known := map[string]bool{}
	for _, c := range s.Customers.Customers {
		known[c.ID] = true
	}
	references := map[string]map[string]bool{}
	add := func(file, customer string) {
		if file != "" && known[customer] {
			if references[file] == nil {
				references[file] = map[string]bool{}
			}
			references[file][customer] = true
		}
	}
	for _, q := range s.Workflow.Quotations {
		add(q.ImageFileID, q.CustomerID)
		for _, id := range q.ImageFileIDs {
			add(id, q.CustomerID)
		}
	}
	for _, o := range s.Workflow.Manufacturing {
		add(o.ImageFileID, o.CustomerID)
		for _, id := range o.ImageFileIDs {
			add(id, o.CustomerID)
		}
	}
	for _, p := range s.Workflow.Payments {
		add(p.ProofFileID, p.CustomerID)
	}
	for index := range s.Files.Files {
		file := &s.Files.Files[index]
		if file.CustomerID == "" || known[file.CustomerID] {
			continue
		}
		owners := references[file.ID]
		if len(owners) != 1 {
			return nil, fmt.Errorf("file %s has an unresolved customer; review its ownership before migration", file.ID)
		}
		for owner := range owners {
			changes := map[string]string{"customerId": owner}
			if file.OwnerID == file.CustomerID {
				changes["ownerId"] = owner
				file.OwnerID = owner
			}
			file.CustomerID = owner
			repairs = append(repairs, repair{"files", index, "Customer recovered from its quotation/payment references", changes})
		}
	}
	// The JSON service rebuilds its index last-row-wins. Keep that public URL intact.
	seen := map[string]bool{}
	all := map[string]bool{}
	for _, file := range s.Files.Files {
		all[file.ID] = true
	}
	for i := len(s.Files.Files) - 1; i >= 0; i-- {
		file := &s.Files.Files[i]
		if !seen[file.ID] {
			seen[file.ID] = true
			continue
		}
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", file.ID, file.StorageKey, i)))
		id := file.ID + "_legacy_" + hex.EncodeToString(sum[:8])
		if all[id] {
			return nil, fmt.Errorf("legacy file ID collision")
		}
		all[id] = true
		changes := map[string]string{"id": id, "url": "/api/files/" + id + "/content"}
		file.ID = id
		file.URL = changes["url"]
		if file.ThumbnailURL != "" {
			changes["thumbnailUrl"] = "/api/files/" + id + "/thumbnail"
			file.ThumbnailURL = changes["thumbnailUrl"]
		}
		repairs = append(repairs, repair{"files", i, "Preserved displaced duplicate-ID record under a unique ID; original URL still resolves to the same file", changes})
	}
	for i := range s.Customers.Requests {
		request := &s.Customers.Requests[i]
		if request.Status != "active" {
			continue
		}
		for _, customer := range s.Customers.Customers {
			if customer.ID != request.CustomerID || customer.VerificationStatus != "verified" {
				continue
			}
			submitted, err := time.Parse(time.RFC3339, customer.DocumentsSubmittedAt)
			if err != nil {
				continue
			}
			created, err := time.Parse(time.RFC3339, request.CreatedAt)
			if err != nil || submitted.Before(created) {
				continue
			}
			request.Status = "submitted"
			request.SubmittedAt = customer.DocumentsSubmittedAt
			request.UsedAt = customer.DocumentsSubmittedAt
			repairs = append(repairs, repair{"documentRequests", i, "Closed stale upload link after recorded submission and account verification", map[string]string{"status": "submitted", "submittedAt": request.SubmittedAt, "usedAt": request.UsedAt}})
		}
	}
	return repairs, nil
}

func validateSource(s source) error {
	known := map[string]bool{}
	for _, c := range s.Customers.Customers {
		known[c.ID] = true
	}
	seen := map[string]bool{}
	for _, file := range s.Files.Files {
		if file.ID == "" || file.StorageKey == "" || file.TenantID != "tenant_chakuchuri" {
			return fmt.Errorf("invalid or unsupported tenant in file metadata")
		}
		if seen[file.ID] {
			return fmt.Errorf("duplicate file ID %s; prepare and review a separate migration copy first", file.ID)
		}
		seen[file.ID] = true
		if file.CustomerID != "" && !known[file.CustomerID] {
			return fmt.Errorf("unknown customer on file %s", file.ID)
		}
	}
	active := map[string]bool{}
	for _, r := range s.Customers.Requests {
		if !known[r.CustomerID] {
			return fmt.Errorf("unknown KYC customer")
		}
		if strings.EqualFold(r.Status, "active") {
			if active[r.CustomerID] {
				return fmt.Errorf("multiple active KYC links for %s; review before migration", r.CustomerID)
			}
			active[r.CustomerID] = true
		}
	}
	return nil
}
