package eventdb

// This file holds the standard ontology, which if possible, we'd like to share
// among all Cyclops servers. Having a standard ontology opens up the possibility
// of sharing training data between servers, and also makes it easier to
// integrate with other systems.

func LoadStandardOntology(e *EventDB) error {
	versionForLogs := 1 // version is just for logs. real "version" is the record ID in the DB.
	std := OntologyDefinition{
		Tags: []OntologyTag{
			{
				Name:  "Intruder",
				Level: OntologyLevelAlarm,
			},
			{
				Name:  "Just Record",
				Level: OntologyLevelRecord,
			},
			{
				Name:  "Ignore",
				Level: OntologyLevelIgnore,
			},
		},
	}

	latest, err := e.FindLatestOntologyThatIsSupersetOf(&std)
	if err != nil {
		return err
	}
	if latest != nil {
		return nil
	}
	e.log.Infof("Creating standard ontology version %v", versionForLogs)

	_, err = e.CreateOntology(&std)
	return err
}
