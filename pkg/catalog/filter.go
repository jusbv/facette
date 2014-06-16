package catalog

import (
	"log"
	"regexp"

	"github.com/facette/facette/pkg/config"
	"github.com/facette/facette/thirdparty/github.com/fatih/set"
)

type filterChain struct {
	Input  chan *CatalogRecord
	output chan *CatalogRecord
	rules  []*config.OriginFilterConfig
}

func newFilterChain(filters []*config.OriginFilterConfig, output chan *CatalogRecord) filterChain {
	chain := filterChain{
		Input:  make(chan *CatalogRecord),
		output: output,
		rules:  make([]*config.OriginFilterConfig, 0),
	}

	targetSet := set.New(set.NonThreadSafe)
	targetSet.Add("any", "origin", "source", "metric")

	for _, filter := range filters {
		if filter.Target == "" {
			filter.Target = "any"
		}

		if !targetSet.Has(filter.Target) {
			log.Printf("ERROR: unknown `%s' filter target", filter.Target)
			continue
		}

		re, err := regexp.Compile(filter.Pattern)
		if err != nil {
			log.Printf("WARNING: unable to compile filter pattern: %s, discarding", err.Error())
			continue
		}

		filter.PatternRegexp = re

		chain.rules = append(chain.rules, filter)
	}

	go func(chain filterChain) {
		for record := range chain.Input {
			// Forward record if no rule defined
			if len(chain.rules) == 0 {
				chain.output <- record
			}

			for _, rule := range chain.rules {
				if (rule.Target == "origin" || rule.Target == "any") && rule.PatternRegexp.MatchString(record.Origin) {
					if rule.Discard {
						log.Printf("DEBUG: discard record %v, as origin matches `%s' pattern", record, rule.Pattern)
						goto nextRecord
					}

					record.Origin = rule.PatternRegexp.ReplaceAllString(record.Origin, rule.Rewrite)
				}

				if (rule.Target == "source" || rule.Target == "any") && rule.PatternRegexp.MatchString(record.Source) {
					if rule.Discard {
						log.Printf("DEBUG: discard record %v, as source matches `%s' pattern", record, rule.Pattern)
						goto nextRecord
					}

					record.Source = rule.PatternRegexp.ReplaceAllString(record.Source, rule.Rewrite)
				}

				if (rule.Target == "metric" || rule.Target == "any") && rule.PatternRegexp.MatchString(record.Metric) {
					if rule.Discard {
						log.Printf("DEBUG: discard record %s, as metric matches `%s' pattern", record, rule.Pattern)
						goto nextRecord
					}

					record.Metric = rule.PatternRegexp.ReplaceAllString(record.Metric, rule.Rewrite)
				}
			}

			chain.output <- record
		nextRecord:
		}
	}(chain)

	return chain
}