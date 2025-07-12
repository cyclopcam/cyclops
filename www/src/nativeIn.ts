// nativeIn exposes entrypoints that our native app (Java/Swift) uses to talk to us.

//import { setBearerToken } from "./auth";
import { globals } from "./globals";
import { router } from "./router/routes";
import { popRoute, replaceRoute } from "./router/helpers";

// Back/Forward in history
(window as any).cyBack = () => {
	if (!popRoute(router)) {
		console.log("cyBack (history stack is empty)");
		return false;
	}
	console.log("cyBack (route popped)");
	return true;
};

// Set app mode
(window as any).cyActivateAppMode = () => {
	globals.setAppMode();
	console.log("App mode activated (JS)");
};

// Set progress message, for a long-running native operation.
// If the string starts with "ERROR:" then it's an error message, and we'll strip out the "ERROR:" prefix.
(window as any).cySetProgressMessage = (message: string) => {
	globals.nativeProgressMessage = message;
};

(window as any).cySetIdentityToken = (token: string) => {
	globals.nativeIdentityToken = token;
};

// Handle a notification. This is the user clicking on the notification on the phone.
// This notification was initially sent by us. It could have been sent seconds or hours ago.
// If the app was asleep when the notification arrives, then it will place the notification ID in the URL query string,
// and get handled in the globals constructor.
(window as any).cyHandleNotification = (notificationId: number) => {
	globals.notificationId = notificationId;
	replaceRoute(router, { name: "rtMonitor" });
};

// Experiment with Message Ports. Not used yet.
// Setup listener for message ports, which I learned about after having built the various other cyXXX functions.
// I'm not sure if there's a practical/performance difference between these two methods of communication.
// ahh.. This is useful if the native side needs to send us a non-trivial amount of data.
(window as any).addEventListener('message', (event: any) => {
	const port = event.ports[0];
	if (port) {
		console.log("Setting up event listener from native port");

		port.onmessage = (event: any) => {
			console.log('Message from Android:', event.data);
		};

		// Send a message back to Android
		port.postMessage('Hello from JS');
	}
});