import { globals } from "@/globals";
import { fetchOrErr } from "@/util/util";

export type EventType = "arm" | "disarm" | "alarm"; // SYNC-EVENT-TYPES
export type AlarmType = "camera-object" | "panic"; // SYNC-ALARM-TYPES

export interface EventDetailAlarm {
	alarmType: AlarmType; // Type of alarm (eg camera object, panic)
	cameraId: number; // ID of the camera that triggered the alarm
}

export interface EventDetailArm {
	userId: number; // ID of the user
	deviceId: string; // ID of the device that armed/disarmed the system (eg phone ID)
}

export interface EventDetail {
	arm?: EventDetailArm; // Must be populated for EventTypeArm and EventTypeDisarm
	alarm?: EventDetailAlarm; // Must be populated for EventTypeAlarm
}

// SYNC-EVENT
export interface SystemEvent {
	id: number; // Unique ID of the event
	time: number; // Time of the event (unix milliseconds)
	eventType: EventType; // Type of event (eg arm, disarm, alarm)
	detail: EventDetail; // Details of the event, can be arm or alarm
}

export async function fetchEvent(id: number): Promise<SystemEvent | null> {
	let res = await fetchOrErr(`/api/events/${id}`);
	if (!res.ok) {
		globals.networkError = res.error;
		return null;
	}
	let j = await res.r.json();
	return j as SystemEvent;
}


/*
export type EventType = "arm" | "disarm" | "alarm"; // SYNC-EVENT-TYPES

export type AlarmType = "camera-object" | "panic"; // SYNC-ALARM-TYPES

export class EventDetailAlarm {
	constructor(public alarmType: AlarmType, public cameraId: number) { }
}

export class EventDetailArm {
	constructor(public userId: number, public deviceId: string) { }
}

// SYNC-EVENT
export class Event {
	id = 0;
	time = new Date();
	eventType: EventType = "arm";
	arm: EventDetailArm | null = null; // Only populated for arm/disarm events
	alarm: EventDetailAlarm | null = null; // Only populated for alarm events

	static async fetchEvent(id: number): Promise<Event | null> {
		let res = await fetchOrErr(`/api/events/${id}`);
		if (!res.ok) {
			globals.networkError = res.error;
			return null;
		}
		let j = await res.r.json();
		let ev = new Event();
		ev.id = j.id;
		ev.time = new Date(j.time);
		ev.eventType = j.eventType;
		if (j.detail) {
			if (j.eventType === "arm") {
				ev.arm = new EventDetailArm(j.detail.userId, j.detail.deviceId);
			} else if (j.eventType === "alarm") {
				ev.alarm = new EventDetailAlarm(j.detail.alarmType, j.detail.cameraId);
			}
		}
		return ev;
	}
}
*/