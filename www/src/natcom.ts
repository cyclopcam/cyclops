import { setBearerToken } from "./auth";
import { globals } from "./globals";
import { popRoute } from "./router/routes";
// natcom is for native communication with our mobile app

// Back/Forward in history
(window as any).cyBack = () => {
	if (!popRoute()) {
		return false;
	}
	return true;
};

// Set app mode
(window as any).cyActivateAppMode = () => {
	globals.isApp = true;
};

// Ensure that we have credentials for this server
(window as any).cySetCredentials = (publicKey: string, bearerToken: string) => {
	setBearerToken(publicKey, bearerToken);
};
