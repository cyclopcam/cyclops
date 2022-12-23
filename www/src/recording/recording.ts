import { encodeQuery, fetchOrErr, type FetchResult } from "@/util/util";
import type { OrError } from "@/util/util";

export class Recording {
	id = 0;
	startTime = new Date();
	ontology: Ontology | null = null;
	labels: Labels | null = null;
	useForTraining = false;

	static fromJSON(j: any, idToOntology: { [key: number]: Ontology } | null): Recording {
		let r = new Recording();
		r.id = j.id;
		r.startTime = new Date(j.startTime);
		r.useForTraining = j.useForTraining ?? false;
		if (idToOntology) {
			r.ontology = idToOntology[j.ontologyID] || null;
		}
		if (j.labels) {
			r.labels = new Labels();
			r.labels.videoTags = j.labels.videoTags;
		}
		return r;
	}

	toJSON(): any {
		return {
			id: this.id,
			startTime: this.startTime.getTime(),
			ontologyID: this.ontology?.id,
			labels: this.labels,
			useForTraining: this.useForTraining,
		};
	}

	static async fetch(ontologies?: Ontology[], id?: number): Promise<OrError<Recording[]>> {
		if (!ontologies) {
			let ontologiesFetch = await Ontology.fetch();
			if (!ontologiesFetch.ok) {
				return ontologiesFetch;
			}
			ontologies = ontologiesFetch.value;
		}
		let idToOntology = Object.fromEntries(ontologies.map((x) => [x.id, x]));

		// Fetch recordings
		let params: any = {};
		if (id) {
			params["id"] = id;
		}
		let r = await fetchOrErr("/api/record/getRecordings?" + encodeQuery(params));
		if (!r.ok) {
			return { ok: false, err: r.error };
		}
		let recordings = ((await r.r.json()) as any[]).map((x) => Recording.fromJSON(x, idToOntology));

		return { ok: true, value: recordings };
	}

	async saveLabels(): Promise<FetchResult> {
		return fetchOrErr("/api/record/setLabels", { method: "POST", body: JSON.stringify(this.toJSON()) });
	}
}

// Labels on a video
export class Labels {
	videoTags: number[] = []; // indices refer to Ontology.videoTags
	cropStart: number = 0; // video crop start time in seconds
	cropEnd: number = 0; // video crop end time in seconds

	fromJSON(j: any): Labels {
		let labels = new Labels();
		labels.videoTags = j.videoTags;
		labels.cropStart = j.cropStart;
		labels.cropEnd = j.cropEnd;
		return labels;
	}
}

export enum OntologyLevel {
	// SYNC-ONTOLOGY-LEVEL
	Alarm = "alarm", // If the system is armed, trigger an alarm
	Record = "record", // Record this incident, whether armed or not
	Ignore = "ignore", // Do not record
}

export function severity(level: OntologyLevel): number {
	switch (level) {
		case OntologyLevel.Alarm:
			return 2;
		case OntologyLevel.Record:
			return 1;
		case OntologyLevel.Ignore:
			return 0;
	}
}

export class OntologyTag {
	name = "";
	level = OntologyLevel.Record;

	constructor(name: string, level: OntologyLevel) {
		this.name = name;
		this.level = level;
	}

	get severity(): number {
		return severity(this.level);
	}

	static fromJSON(j: any): OntologyTag {
		return new OntologyTag(j.name, j.level);
	}

	toJSON() {
		return {
			name: this.name,
			level: this.level,
		};
	}
}

export class Ontology {
	id = 0;
	createdAt = new Date();
	tags: OntologyTag[] = []; // allowable video tags. In Recording records, tags are zero-based indices into this array.

	static fromJSON(j: any): Ontology {
		let o = new Ontology();
		o.id = j.id;
		o.createdAt = new Date(j.createdAt);
		if (j.definition) {
			if (j.definition.tags) {
				o.tags = j.definition.tags.map((x: any) => OntologyTag.fromJSON(x));
			}
		}
		return o;
	}

	toJSON(): any {
		return {
			id: this.id,
			createdAt: this.createdAt.getTime(),
			definition: {
				tags: this.tags.map((x) => x.toJSON()),
			},
		};
	}

	static async fetch(): Promise<OrError<Ontology[]>> {
		let r = await fetchOrErr("/api/record/getOntologies");
		if (!r.ok) {
			return { ok: false, err: r.error };
		}
		let onto = ((await r.r.json()) as any[]).map((x) => Ontology.fromJSON(x));
		return { ok: true, value: onto };
	}

	static latest(list: Ontology[]): Ontology | null {
		if (list.length === 0) {
			return null;
		}
		let m = 0;
		let best = null;
		for (let o of list) {
			if (o.createdAt.getTime() > m) {
				m = o.createdAt.getTime();
				best = o;
			}
		}
		return best;
	}
}
