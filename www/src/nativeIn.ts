// nativeIn exposes entrypoints that our native app (Java/Swift) uses to talk to us.

//import { setBearerToken } from "./auth";
import { globals } from "./globals";
import { popRoute } from "./router/routes";

// Back/Forward in history
(window as any).cyBack = () => {
	if (!popRoute()) {
		console.log("cyBack (history stack is empty)");
		return false;
	}
	console.log("cyBack (route popped)");
	return true;
};

// Set app mode
(window as any).cyActivateAppMode = () => {
	globals.isApp = true;
	console.log("App mode activated (JS)");
};

// Ensure that we have credentials for this server
//(window as any).cySetCredentials = (publicKey: string, bearerToken: string) => {
//	setBearerToken(publicKey, bearerToken);
//};
