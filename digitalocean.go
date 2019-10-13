package pinamicdns

import (
	"context"
	"errors"
	"net"

	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
	"golang.org/x/xerrors"
)

const ARecordType = "A"

var (
	errNoRecordsFound = errors.New("no existing record found")
	errNoUpdateNeeded = errors.New("no update is needed to bring the record up to date")
)

// DigitalOceanIPSetter is an IPSetter that will update records in DigitalOcean's DNS
type DigitalOceanIPSetter struct {
	tokenSource oauth2.TokenSource
	recordTTL   int
}

// digitalOceanTransaction holds all elements necessary to talk to the DigitalOcean API, in the context of a single
// DigitalOceanIPSetter.SetIP call.
type digitalOceanTransaction struct {
	ctx    context.Context
	client *godo.Client
}

// DigitalOceanRecordTTL should be passed to NewDigitalOceanIPSetter if a TTL is desired for the records it sets
func DigitalOceanRecordTTL(ttl int) func(*DigitalOceanIPSetter) error {
	return func(setter *DigitalOceanIPSetter) error {
		setter.recordTTL = ttl
		return nil
	}
}

// getUpdatableRecord gets a single A record to update from DigtialOcean. The new record must have the same name and a
// different value than what is given. If all existing records carry the same value, errNoUpdateNeeded is returned. If
// no record exists to be updated, errNoRecordsFound is returned.
func (transaction digitalOceanTransaction) getUpdatableARecord(domain, name, proposedValue string) (godo.DomainRecord, error) {
	records, res, err := transaction.client.Domains.Records(transaction.ctx, domain, nil)
	if err != nil {
		return godo.DomainRecord{}, xerrors.Errorf("could not ask DigitalOcean API for records: %w", err)
	} else if resErr := godo.CheckResponse(res.Response); resErr != nil {
		return godo.DomainRecord{}, xerrors.Errorf("could not ask DigitalOcean API for records: %w", resErr)
	}

	// Represents whether or not we have a record that has the same name
	haveName := false
	for _, record := range records {
		// Records with a differing name or non A records are invalid.
		if record.Type != ARecordType || record.Name != name {
			continue
		} else if record.Name == name {
			// If we have a record with the same name, notate it as such
			haveName = true
		}

		if record.Data != proposedValue {
			return record, nil
		}
	}

	// If we have a record with the same name and we haven't returned, it must have the same value as what is propsoed.
	// If this is the case, no update is needed
	if haveName {
		return godo.DomainRecord{}, errNoUpdateNeeded
	}

	return godo.DomainRecord{}, errNoRecordsFound
}

// createRecord creates a DNS record for the given domain, in correspondence with the given DomainRecordEditRequest
func (transaction digitalOceanTransaction) createRecord(domain string, editRequest godo.DomainRecordEditRequest) error {
	_, res, err := transaction.client.Domains.CreateRecord(transaction.ctx, domain, &editRequest)
	if err != nil {
		return xerrors.Errorf("could not create record for domain: %w", err)
	} else if resErr := godo.CheckResponse(res.Response); resErr != nil {
		return xerrors.Errorf("could not create record for domain: %w", resErr)
	}

	return nil
}

// updateRecord updates an existing DNS record for the given domain, in correspondence with the given DomainRecordEditRequest
func (transaction digitalOceanTransaction) updateRecord(domain string, existingRecord godo.DomainRecord, editRequest godo.DomainRecordEditRequest) error {
	_, res, err := transaction.client.Domains.EditRecord(transaction.ctx, domain, existingRecord.ID, &editRequest)
	if err != nil {
		return xerrors.Errorf("could not update record for domain: %w", err)
	} else if resErr := godo.CheckResponse(res.Response); resErr != nil {
		return xerrors.Errorf("could not update record for domain: %w", resErr)
	}

	return nil
}

// NewDigitalOceanIPSetter makes a new DigitalOcean IPSetter
func NewDigitalOceanIPSetter(tokenSource oauth2.TokenSource, options ...func(*DigitalOceanIPSetter) error) (DigitalOceanIPSetter, error) {
	setter := DigitalOceanIPSetter{
		tokenSource: tokenSource,
	}

	for _, option := range options {
		err := option(&setter)
		if err != nil {
			return DigitalOceanIPSetter{}, xerrors.Errorf("could not construct DigitalOceanIPSetter: %w", err)
		}
	}

	return setter, nil
}

// getDigitalOceanClient will make a new Digital Ocean API transaction for the given setter.
func (setter DigitalOceanIPSetter) makeTransaction(ctx context.Context) digitalOceanTransaction {
	oauth2Client := oauth2.NewClient(ctx, setter.tokenSource)

	return digitalOceanTransaction{
		ctx:    ctx,
		client: godo.NewClient(oauth2Client),
	}
}

// SetIP associates the given ip with the given domain and subdomain name, in the form of a DNS record with DigitalOcean.
func (setter DigitalOceanIPSetter) SetIP(domain, name string, ip net.IP) error {
	ctx := context.Background()
	transaction := setter.makeTransaction(ctx)
	editRequest := makeARecordEditRequest(name, ip, setter.recordTTL)
	existingRecord, err := transaction.getUpdatableARecord(domain, name, ip.String())
	// setErr holds an error associated with setting the address, once a method has been determined.
	var setErr error
	if err == errNoUpdateNeeded {
		return nil
	} else if err == errNoRecordsFound {
		setErr = transaction.createRecord(domain, editRequest)
	} else {
		setErr = transaction.updateRecord(domain, existingRecord, editRequest)
	}

	if setErr != nil {
		return xerrors.Errorf("Could not set IP: %w", setErr)
	}

	return nil
}

// makeARecordEditRequest makes an edit request for an A record pointing to the given ip at the given subdomain.
func makeARecordEditRequest(name string, ip net.IP, ttl int) godo.DomainRecordEditRequest {
	return godo.DomainRecordEditRequest{
		Type: ARecordType,
		Name: name,
		Data: ip.String(),
		TTL:  ttl,
	}
}
