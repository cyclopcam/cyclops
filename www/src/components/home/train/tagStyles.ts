import { OntologyLevel, type OntologyTag } from "@/recording/recording";

// See tag.scss
export function tagColorClasses(tag: OntologyTag | null): { [key: string]: boolean } {
	if (!tag) {
		return { "unlabeledBG": true };
	}
	switch (tag.level) {
		case OntologyLevel.Ignore: return { "ignoreBG": true };
		case OntologyLevel.Record: return { "recordBG": true };
		case OntologyLevel.Alarm: return { "alarmBG": true };
	}
}