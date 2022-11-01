import { ref, watch } from "vue";

// Maintain visibility of passwords as default state throughout the entire application
let showPasswordsState: { [index: string]: boolean } = {};

export interface Options {
	inputWidth?: string;
	inputColor?: string; // Color of input text fields
	name?: string; // Name of the form. Used to remember password visibility preference
	submitTitle?: string; // Title of the submit button
}

export class Context {
	// validate returns true if the form is ready to be submitted to the server.
	// normally, you'll just check that all required values are populated.
	// validate must be fast, because once the user has clicked Next, it will be triggered on every input change
	constructor(validate: () => boolean, options?: Options) {
		this.validate = validate;
		if (options?.name) this.name = options.name;
		if (options?.inputWidth) this.inputWidth = ref(options.inputWidth);
		if (options?.inputColor) this.inputColor = ref(options.inputColor);
		if (options?.submitTitle) this.submitTitle = ref(options.submitTitle);

		if (this.name) {
			this.showPasswords.value = showPasswordsState[this.name] || false;
		}

		watch(this.showPasswords, (newVal, oldVal) => {
			if (this.name) {
				showPasswordsState[this.name] = newVal;
			}
		});
	}

	validate: () => boolean;

	inputWidth = ref("180px");
	inputColor = ref("#000");
	name = ""; // Name of the form. Used for persistent state (eg showPassword)
	submitTitle = ref("Next"); // Title of the submit button

	submitClicked = ref(false); // Set to true (and remains true) after the user clicks the Next button
	anyFailures = ref(false); // Set to true (and remains true) after the user clicks the Next button, and validation fails
	submitError = ref(""); // If we fail to submit, this is the error
	submitBusyMsg = ref(""); // Used to inform user of submit status
	idToError = ref({} as { [key: string]: string }); // Map from id to error string, to show error beneath a specific item
	busy = ref(false); // true while we're waiting for server confirmation of a form action (eg submit)
	showPasswords = ref(false); // Toggle visibility of passwords

	invokeSubmitOnEnter = ref(false); // A watcher on FormBottom waits for this to become true, and then simulates clicking the Submit button

	get showRequiredDots(): boolean {
		return this.anyFailures.value;
	}

	get showCompleteAllFields(): boolean {
		let isValid = this.validate();
		if (this.submitClicked.value && !isValid) {
			this.anyFailures.value = true;
		}
		return this.submitClicked.value && !isValid;
	}
}
