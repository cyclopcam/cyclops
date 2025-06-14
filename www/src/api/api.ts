// This is a grab-bag of various API definitions from the server, which don't seem to belong anywhere else

// SYNC-SYSTEM-INFO-JSON
export interface SystemInfoJSON {
	startupErrors: StartupErrorJSON[];
	cameras: any[]; // CameraInfo.fromJSON() takes these as input
	objectClasses: string[];   // Classes of objects detected by our neural network(s) (eg person, car, truck,...)
	abstractClasses: { [key: string]: string }; // Abstract classes of objects detected by our neural network(s) eg {"car":"vehicle", "truck":"vehicle"}
	lanAddresses: string[]; // List of LAN addresses that the server might be reachable on.
}

// SYNC-STARTUP-ERROR
export interface StartupErrorJSON {
	// SYNC-STARTUP-ERROR-CODES
	code: "ARCHIVE_PATH"; // The only code defined so far.
	message: string;
}
