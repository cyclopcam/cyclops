<script setup lang="ts">
import MobileFullscreen from '@/components/responsive/MobileFullscreen.vue';
import NewUser from '@/components/settings/NewUser.vue';
import { onMounted, ref, watch } from 'vue';
import { globals } from '@/globals';
import { replaceRoute } from "@/router/helpers";
import { parseQuery, useRouter } from 'vue-router';
import WideRoot from '@/components/widewidgets/WideRoot.vue';
import WideInput from '@/components/widewidgets/WideInput.vue';
import WideSection from '@/components/widewidgets/WideSection.vue';
import InfoBubble from '@/components/widgets/InfoBubble.vue';
import { Permissions, UserRecord } from '@/db/config/configdb';
import { encodeQuery, fetchOrErr } from '@/util/util';
import Buttin from '@/components/core/Buttin.vue';
import { natRequestOAuthLogin, OAuthLoginPurpose } from '@/nativeOut';
import NativeProgress from '@/components/widgets/NativeProgress.vue';
import type { RefSymbol } from '@vue/reactivity';
import { handleLoginSuccess } from '@/auth';

const router = useRouter();

let identityType = ref("google");
let username = ref("");
let password = ref("");
let isBusy = ref(false);
let isPhase2 = ref(false);

async function moveToNextStage() {
	await globals.postAuthenticateLoadSystemInfo(false);
	replaceRoute(router, { name: "rtSettingsHome" });
}

function isNextEnabled(): boolean {
	if (identityType.value === "username") {
		return username.value.length > 0 && password.value.length > 0;
	}
	return true;
}

async function onCreateInitialUser() {
	isBusy.value = true;
	if (identityType.value === "google" || identityType.value === "microsoft") {
		natRequestOAuthLogin(OAuthLoginPurpose.InitialUser, identityType.value);
	} else {
		let user = new UserRecord();
		user.username = username.value;
		user.permissions = Permissions.Admin;
		let r = await fetchOrErr('/api/auth/createUser?' + encodeQuery({ password: password.value }), { method: 'POST', body: JSON.stringify(user.toJSON()) });
		// TODO: finish this code path
	}
}

watch(() => globals.nativeIdentityToken, async () => {
	globals.nativeProgressMessage = "Sharing identity token with your server...";
	let r = await fetchOrErr('/api/auth/createUser?' + encodeQuery({ identityToken: globals.nativeIdentityToken }), { method: 'POST' });
	isBusy.value = false;
	if (!r.ok) {
		globals.nativeProgressMessage = "ERROR:" + r.status;
	} else {
		let err = await handleLoginSuccess(r);
		if (err !== "") {
			globals.nativeProgressMessage = "ERROR:" + err;
		}
	}
})

onMounted(async () => {
	// After doing an OAuth login itself, the native app will navigate to us with a URL like this:
	// http://IP:80/welcome?have_accounts_token=1?provider=google
	console.log("parsing window location.search", window.location.search);
	let query = parseQuery(window.location.search);
	console.log("parsing OK");
	if (query.have_accounts_token === "1") {
		// Here we basically just show the same screen that the user was on before we started the OAuth signin flow.
		isPhase2.value = true;
		identityType.value = query.provider as string;
		isBusy.value = true;
		globals.nativeProgressMessage = "Acquiring Identity Token...";
		onCreateInitialUser();
		// After this point, we expect a native call that modifies globals.nativeIdentityToken.
	}

	await globals.waitForPublicKeyLoad();
	await globals.waitForSystemInfoLoad();
})

</script>

<template>
	<wide-root>
		<wide-section>
			<div class="flexColumnCenter">
				<h2 style="text-align: center; margin: 30px 10px">Cyclops System Setup</h2>
				<p class="instruction">Let's create an admin user that will have full control of the system.</p>
				<!-- <p class="note" style="text-align:center">You will need to grant access to subsequent users</p>-->
				<!-- <p class="instruction2">How do you want to login?</p>-->
				<div style="margin: 20px">
					<div class="flexRowCenter">
						<label><input v-model="identityType" value="google" name="identityType" type="radio">Login with
							Google
						</label>
					</div>
					<div class="flexRowCenter">
						<label><input v-model="identityType" value="microsoft" name="identityType" type="radio">Login
							with
							Microsoft</label>
					</div>
					<div class="flexRowCenter">
						<label><input v-model="identityType" value="username" name="identityType" type="radio">Create a
							username and password</label>
					</div>
				</div>
			</div>
			<div v-if="identityType === 'username'" style="padding: 0px 20px">
				<wide-input v-model="username" label="Username" :required="true" okText="OK"></wide-input>
				<wide-input v-model="password" label="Password" :required="true" type="password" okText="OK"
					:bottom-border="true"></wide-input>
				<p class="note">If you lose the password, you won't be able to login, and you'll need root linux access
					to reset the password.</p>
			</div>
			<div class="bottom">
				<buttin :focal="true" :busy="isBusy" :disabled="!isNextEnabled()" @click="onCreateInitialUser">Next
				</buttin>
				<!-- <button class="focalButton" :disabled="!isNextEnabled()" @click="onCreateInitialUser">Next</button> -->
			</div>
			<div class="bottomStatus blinkingStatusText">
				<native-progress :text="globals.nativeProgressMessage" />
			</div>
		</wide-section>
	</wide-root>
	<!--
		<div :class="{ username: true, usernameShrunk: identityType !== 'username' }">
			<label><input></label>
			<new-user :is-first-user="true" @finished="moveToNextStage()" />
		</div>
		-->
</template>

<style lang="scss" scoped>
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
	margin: 20px 20px;
}

label {
	margin: 8px 0 0 0;
	font-size: 18px;
}

input[type="radio"] {
	margin: 0 8px 0 0;
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
