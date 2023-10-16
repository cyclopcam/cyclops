import type { Ref } from "vue";
import { encodeQuery, fetchOrErr, type FetchResult } from "@/util/util";
import type { OrError } from "@/util/util";
import { globals } from "@/globals";

export class Recording {
	id = 0;
	startTime = new Date();
	ontologyID: number | null = null;
	labels: Labels | null = null;
	useForTraining = false;

	static fromJSON(j: any): Recording {
		let r = new Recording();
		r.id = j.id;
		r.startTime = new Date(j.startTime);
		r.useForTraining = j.useForTraining ?? false;
		r.ontologyID = j.ontologyID;
		if (j.labels) {
			r.labels = Labels.fromJSON(j.labels);
		}
		return r;
	}

	toJSON(): any {
		return {
			id: this.id,
			startTime: this.startTime.getTime(),
			ontologyID: this.ontologyID,
			labels: this.labels?.toJSON(),
			useForTraining: this.useForTraining,
		};
	}

	static async fetch(id: number): Promise<OrError<Recording>> {
		let r = await this.fetchList(id);
		if (!r.ok) {
			return r;
		}
		if (r.value.length == 0) {
			return { ok: false, err: "Recording not found" };
		}
		return { ok: true, value: r.value[0] };
	}

	static async fetchAll(): Promise<OrError<Recording[]>> {
		let r = await fetchOrErr("/api/record/getRecordings");
		if (!r.ok) {
			return { ok: false, err: r.error };
		}
		let recordings = ((await r.r.json()) as any[]).map((x) => Recording.fromJSON(x));

		return { ok: true, value: recordings };
	}

	// Fetch either all, or just one recording
	static async fetchList(id?: number): Promise<OrError<Recording[]>> {
		let params: any = {};
		if (id) {
			params["id"] = id;
		}
		let r = await fetchOrErr("/api/record/getRecordings?" + encodeQuery(params));
		if (!r.ok) {
			return { ok: false, err: r.error };
		}
		let recordings = ((await r.r.json()) as any[]).map((x) => Recording.fromJSON(x));

		return { ok: true, value: recordings };
	}

	async saveLabels(): Promise<FetchResult> {
		return fetchOrErr("/api/record/setLabels", { method: "POST", body: JSON.stringify(this.toJSON()) });
	}

	async uploadToArc(): Promise<FetchResult> {
		return fetchOrErr(`/api/record/sendToArc/${this.id}`, { method: "POST" });
	}
}

// Labels on a video
export class Labels {
	videoTags: number[] = []; // indices refer to Ontology.videoTags
	cropStart: number = 0; // video crop start time in seconds
	cropEnd: number = 0; // video crop end time in seconds

	static fromJSON(j: any): Labels {
		let labels = new Labels();
		labels.videoTags = j.videoTags;
		labels.cropStart = j.cropStart;
		labels.cropEnd = j.cropEnd;
		return labels;
	}

	toJSON(): any {
		return {
			videoTags: this.videoTags,
			cropStart: this.cropStart,
			cropEnd: this.cropEnd,
		};
	}
}

export enum OntologyLevel {
	// SYNC-ONTOLOGY-LEVEL
	Alarm = "alarm", // If the system is armed, trigger an alarm
	Record = "record", // Record this incident, whether armed or not
	Ignore = "ignore", // Do not record
}

// This is just here for sorting
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

	static async fetchLatest(): Promise<OrError<Ontology>> {
		let r = await fetchOrErr("/api/record/getLatestOntology");
		if (!r.ok) {
			return { ok: false, err: r.error };
		}
		let onto = Ontology.fromJSON((await r.r.json()));
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

	// Fetch the list of ontologies, and populate the two parameters with the result.
	// On failure, set the global networkError.
	static fetchIntoReactive(ontologies: Ref<Ontology[]>, latestOntology: Ref<Ontology>) {
		Ontology.fetch().then((r) => {
			if (!r.ok) {
				globals.networkError = r.err;
				return;
			}
			ontologies.value = r.value;
			// We expect the server to ensure that there's always at least one ontology record
			let latest = Ontology.latest(r.value);
			if (latest)
				latestOntology.value = latest;
		});
	}
}
