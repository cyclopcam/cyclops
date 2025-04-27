<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';
import { globals } from '@/globals';
import { replaceRoute } from "@/router/helpers";
import { parseQuery, useRouter } from 'vue-router';
import WideRoot from '@/components/widewidgets/WideRoot.vue';
import WideInput from '@/components/widewidgets/WideInput.vue';
import WideSection from '@/components/widewidgets/WideSection.vue';
import { Permissions, UserRecord } from '@/db/config/configdb';
import { encodeQuery, fetchErrorMessage, fetchOrErr, sleep, type FetchResult } from '@/util/util';
import Buttin from '@/components/core/Buttin.vue';
import { natPostLogin, natRequestOAuthLogin, OAuthLoginPurpose } from '@/nativeOut';
import NativeProgress from '@/components/widgets/NativeProgress.vue';
import { handleLoginSuccess } from '@/auth';

const router = useRouter();

let identityType = ref("google");
let username = ref("");
let password = ref("");
let isBusy = ref(false);
let isLoadingInitialState = ref(true);
let isNewSystem = ref(false);
let isRestarting = ref(false);
let restartStartedAt = ref(Date.now());
let secondsSinceRestart = ref(0);
let maxRestartTime = 20;
let restartError = ref("");

function isValidIdentityType(t: any): boolean {
	return t === 'google' || t === 'microsoft' || t === 'username';
}

function isOAuth(): boolean {
	return identityType.value === "google" || identityType.value === "microsoft";
}

function usernamePrompt(): string {
	if (isNewSystem.value) {
		return "Create a username and password";
	} else {
		return "Login with a username and password";
	}
}

async function moveToNextStage() {
	console.log("WelcomeView moveToNextStage");
	if (globals.isApp) {
		// Tell the app to officially switch to our server.
		// This will cause the app to reload us.
		// This is the final step in our login process in the app.
		// In the Android app, this will call switchToServer()
		await natPostLogin();
	} else {
		await globals.postAuthenticateLoadSystemInfo(false);
		if (isNewSystem.value || globals.cameras.length === 0)
			replaceRoute(router, { name: "rtSettingsHome" });
		else
			replaceRoute(router, { name: "rtMonitor" });
	}
}

function isNextEnabled(): boolean {
	if (identityType.value === "username") {
		return username.value.length > 0 && password.value.length > 0;
	}
	return true;
}

function allowOAuth(): boolean {
	return globals.isApp;
}

async function onLogin() {
	isBusy.value = true;
	if (isOAuth()) {
		// The first time this is called, if the native app is not logged into accounts.cyclopcam.org, then it will
		// use a custom chrome tab to initiate the Oauth login flow. Once that's done, it will re-open this page,
		// and inform us that have_accounts_token=1. Then we'll kick off this function again, and this time
		// the native app will acquire a short-lived identity token, and pass it to us via globals.nativeIdentityToken.
		// Our watcher (in this source file) will notice nativeIdentityToken changing, an will send that on to our server.
		await natRequestOAuthLogin(OAuthLoginPurpose.InitialUser, identityType.value);
	} else {
		if (isNewSystem.value)
			await finalCreateUser();
		else
			await finalLogin();
	}
}

function loginMode(): string {
	if (globals.isApp) {
		return "CookieAndBearerToken";
	}
	return "Cookie";
}

// The final step (for initial setup)
async function finalCreateUser() {
	console.log("WelcomeView finalCreateUser 1");

	let r: FetchResult;
	isBusy.value = true;
	if (isOAuth()) {
		r = await fetchOrErr('/api/auth/createUser?' + encodeQuery({ identityToken: globals.nativeIdentityToken }), { method: 'POST' });
	} else {
		let user = new UserRecord();
		user.username = username.value;
		user.permissions = Permissions.Admin;
		r = await fetchOrErr('/api/auth/createUser?' + encodeQuery({ password: password.value }), { method: 'POST', body: JSON.stringify(user.toJSON()) });
	}

	console.log("WelcomeView finalCreateUser 2");

	isBusy.value = false;
	postLoginOrCreateUser(r);
}

// The final step for login
async function finalLogin() {
	if (globals.serverPublicKey === '') {
		globals.nativeProgressMessage = "ERROR:Server failed to validate its public key";
		return;
	}

	let r: FetchResult;

	isBusy.value = true;
	if (isOAuth()) {
		r = await fetchOrErr('/api/auth/login?' + encodeQuery({ loginMode: loginMode() }), { method: 'POST', headers: { "Authorization": "IdentityToken " + globals.nativeIdentityToken } });
	} else {
		let basic = btoa(username.value.trim() + ":" + password.value.trim());
		r = await fetchOrErr('/api/auth/login?' + encodeQuery({ loginMode: loginMode() }), { method: 'POST', headers: { "Authorization": "BASIC " + basic } });
	}
	isBusy.value = false;

	postLoginOrCreateUser(r);
}

async function postLoginOrCreateUser(r: FetchResult) {
	console.log("WelcomeView postLoginOrCreateUser", r.ok);
	if (!r.ok) {
		globals.nativeProgressMessage = "ERROR:" + fetchErrorMessage(r);
	} else {
		// Send cookies etc to native app
		let loginResult = await handleLoginSuccess(r);
		console.log("WelcomeView postLoginOrCreateUser", loginResult);
		if (!loginResult.ok) {
			globals.nativeProgressMessage = "ERROR:" + loginResult.error;
		} else {
			// After initial login with OAuth, the system should be authorized to use the VPN, and acquire
			// an SSL certificate. But it needs to restart to do that.
			if (loginResult.response.needRestart) {
				console.log("Restarting system after login");
				let restartOK = await restartSystem();
				if (!restartOK) {
					console.error("Restart failed: ", restartError.value);
					globals.nativeProgressMessage = "ERROR:" + restartError.value;
					return;
				}
			}
			moveToNextStage();
		}
	}
}

async function restartSystem(): Promise<boolean> {
	isRestarting.value = true;
	restartStartedAt.value = Date.now();
	secondsSinceRestart.value = 0;

	// I don't know what to do if restart fails!
	// This basically cannot be allowed to happen.

	let r = await fetchOrErr('/api/system/restart', { method: 'POST' });
	if (!r.ok) {
		// eek!
		restartError.value = "Failed to initiate restart: " + fetchErrorMessage(r);
		isRestarting.value = false;
		return false;
	}

	// Add an extra sleep, purely for my own debugging sanity!
	await sleep(1000);

	while (secondsSinceRestart.value < maxRestartTime) {
		secondsSinceRestart.value = Math.floor((Date.now() - restartStartedAt.value) / 1000);
		await sleep(1000);
		let r = await fetchOrErr('/api/ping');
		if (r.ok) {
			isRestarting.value = false;
			return true;
		}
	}

	restartError.value = "Restart timed out";
	isRestarting.value = false;
	return false;
}

watch(() => globals.nativeIdentityToken, async () => {
	// Native app has shared a short-lived identity token with us, so use it to create a user and/or login.
	globals.nativeProgressMessage = "Sharing identity token with your server...";
	if (isNewSystem.value)
		await finalCreateUser();
	else
		await finalLogin();
})

onMounted(async () => {
	await globals.waitForPublicKeyLoad();
	await globals.waitForSystemInfoLoad();
	isNewSystem.value = !(await (await fetch("/api/auth/hasAdmin")).json() as boolean);
	isLoadingInitialState.value = false;
	console.log("isNewSystem:", isNewSystem.value);

	// After doing an OAuth login itself, the native app will navigate to us with a URL like this:
	// http://<lan ip>:80/welcome?have_accounts_token=1?provider=google
	//console.log("parsing window location.search", window.location.search);
	let query = parseQuery(window.location.search);

	if (isValidIdentityType(query['provider'])) {
		identityType.value = query['provider'] as string;
	}

	//console.log("parsing OK");
	if (query['have_accounts_token'] === "1") {
		// Here we basically just show the same screen that the user was on before we started the OAuth signin flow.
		globals.nativeProgressMessage = "Acquiring Identity Token...";
		await onLogin();
		// After this point, we expect a native call that modifies globals.nativeIdentityToken.
	}
	// Uncomment this to test the restart system code
	//restartSystem();
})

</script>

<template>
	<wide-root>
		<wide-section v-if="isRestarting">
			<div class="flexColumnCenter">
				<h2>Restarting...</h2>
				<p class="instruction">Cyclops is restarting</p>
				<p class="instruction" style="margin:20px 10px 20px 10px">It can take up to 20 seconds for DNS records
					to propagate globally.</p>
				<div style="margin:50px">{{ maxRestartTime - secondsSinceRestart }}</div>
				<div v-if="restartError !== ''" class="error">{{ restartError }}</div>
			</div>
		</wide-section>
		<wide-section v-else>
			<div class="flexColumnCenter">
				<div v-if="isLoadingInitialState">
					<h2>Loading...</h2>
					<p class="instruction">Loading system information.</p>
				</div>
				<div v-else-if="isNewSystem">
					<h2>Cyclops System Setup</h2>
					<p class="instruction">Let's create an admin user that will have full control of the system.</p>
				</div>
				<div v-else>
					<h2>Cyclops Login</h2>
					<!-- <p class="instruction">Enter your credentials to access the system.</p> -->
				</div>
				<div style="margin: 20px">
					<div class="option">
						<input v-model="identityType" id="optionGoogle" value="google" name="identityType" type="radio"
							:disabled="!allowOAuth()">
						<label for="optionGoogle">Login with Google </label>
					</div>
					<div class="methodDisabled" v-if="!allowOAuth()">Only available from Android/iOS App</div>

					<div class="option">
						<input v-model="identityType" id="optionMicrosoft" value="microsoft" name="identityType"
							type="radio" :disabled="!allowOAuth()">
						<label for="optionMicrosoft">Login with Microsoft</label>
					</div>
					<div class="methodDisabled" v-if="!allowOAuth()">Only available from Android/iOS App</div>

					<div class="option">
						<input v-model="identityType" id="optionUsername" value="username" name="identityType"
							type="radio">
						<label for="optionUsername">{{ usernamePrompt() }}</label>
					</div>
				</div>
			</div>
			<div v-if="identityType === 'username'" style="padding: 0px 20px">
				<wide-input v-model="username" label="Username" :required="true" okText="OK"></wide-input>
				<wide-input v-model="password" label="Password" :required="true" type="password" okText="OK"
					:bottom-border="true"></wide-input>
				<!--
				<p v-if="isNewSystem || true" class="note">If you lose the password, you won't be able to login, and
					you'll need root linux access to reset the password.</p>
					-->
				<p v-if="isNewSystem" class="note">By choosing this option, you won't have access to:
				<ul>
					<li>Push Notifications (Alarms)</li>
					<li>VPN access from outside your home network</li>
					<li>Monitoring by armed response security companies</li>
				</ul>
				</p>
			</div>
			<div class="bottom">
				<buttin :focal="true" :busy="isBusy" :disabled="!isNextEnabled()" @click="onLogin">Next
				</buttin>
			</div>
			<div class="bottomStatus">
				<native-progress :text="globals.nativeProgressMessage" />
			</div>
		</wide-section>
	</wide-root>
</template>

<style lang="scss" scoped>
h2 {
	text-align: center;
	margin: 30px 10px 20px 10px;
}

ul {
	padding-left: 30px;
}

.error {
	color: #d00;
	margin: 20px;
}

.instruction {
	text-align: center;
	margin: 0 40px;
	font-weight: 500;
}

.instruction2 {
	margin: 10px 40px;
	font-weight: 500;
}

.note {
	font-size: 14px;
	margin: 25px 15px 15px 15px;
}

.option {
	display: flex;
	align-items: center; // so that we can nicely aligned radio button and label on mobile. desktop looks a bit weird, but who cares.
}

.option input[type="radio"] {
	margin: 6px 8px 6px 0px;
}

.option label {
	font-size: 18px;
}

.methodDisabled {
	color: #888;
	font-size: 12px;
	margin: 4px 0 10px 22px;
}

.bottom {
	margin: 20px 0px 12px 0px;
	display: flex;
	justify-content: right;
}

.bottomStatus {
	margin: 20px 20px;
}

.username {
	transition: transform 0.3s;
	transform: scale(1.0);
}

.usernameShrunk {
	transform: scale(0.0);
}
</style>
