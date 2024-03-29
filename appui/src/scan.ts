import { fakeServerList } from "./nativeOut";

// SYNC-SCANNED-SERVER
export interface ScannedServer {
	ip: string;
	hostname: string;
	publicKey: string;
}

// SYNC-SCAN-STATE	
// LAN IP address scan
export interface ScanState {
	error: string; // If not empty, then status will be "d", and scan has stopped
	phoneIP: string;
	status: "i" | "b" | "d"; // i:initial, b:busy, d:done
	servers: ScannedServer[];
	nScanned: number;
}

export function initialScanState(): ScanState {
	return { error: "", phoneIP: "", status: "i", servers: [], nScanned: 0 };
}

// For progress bar
export const maxIPsToScan = 253;

// Used for UI design without having to do an IP scan
export function mockScanState(ss: ScanState, progress_0_to_1: number) {
	progress_0_to_1 = Math.min(progress_0_to_1, 1);
	ss.phoneIP = "192.168.10.65";
	ss.error = "";
	ss.nScanned = Math.round(progress_0_to_1 * maxIPsToScan);
	ss.servers = [];
	// Keep these in sync with fakeServerList in nattypes.ts
	let venus = fakeServerList.find(x => x.name === "venus")!;
	let mars = fakeServerList.find(x => x.name === "mars")!;
	let molehill = fakeServerList.find(x => x.name === "molehill")!;
	if (progress_0_to_1 >= 0.1) {
		ss.servers.push({ ip: venus.lanIP, hostname: venus.name, publicKey: venus.publicKey });
	}
	if (progress_0_to_1 >= 0.4) {
		ss.servers.push({ ip: mars.lanIP, hostname: mars.name, publicKey: mars.publicKey });
	}
	if (progress_0_to_1 >= 0.7) {
		ss.servers.push({ ip: molehill.lanIP, hostname: molehill.name, publicKey: molehill.publicKey });
	}
	if (progress_0_to_1 >= 1) {
		ss.status = "d";
	} else {
		ss.status = "b";
	}
}

export function mockScanStateError(ss: ScanState) {
	ss.phoneIP = "";
	ss.error = "Android Internal Error, Java foo bar etc etc. Errors are often long. Failed to get WiFi IP address.";
	ss.nScanned = 0;
	ss.servers = [];
	ss.status = "d";
}
