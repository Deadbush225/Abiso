<script>
	import { onMount } from "svelte";

	const BACKEND_URL = import.meta.env.VITE_BACKEND_URL;

	// State management using Svelte reactive variables
	let city = $state("");
	let barangay = $state("");
	let statusMessage = $state("");
	let isButtonDisabled = $state(true);
	let registration = null;

	// Helper to convert base64 VAPID keys for the browser PushManager
	function urlBase64ToUint8Array(base64String) {
		const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
		const base64 = (base64String + padding)
			.replace(/\-/g, "+")
			.replace(/_/g, "/");
		const rawData = window.atob(base64);
		const outputArray = new Uint8Array(rawData.length);
		for (let i = 0; i < rawData.length; ++i) {
			outputArray[i] = rawData.charCodeAt(i);
		}
		return outputArray;
	}

	onMount(async () => {
		if ("serviceWorker" in navigator && "PushManager" in window) {
			try {
				// Accessing the sw.js hosted out of your public directory
				registration = await navigator.serviceWorker.register("/sw.js");
				isButtonDisabled = false;
				statusMessage = "System ready.";
			} catch (err) {
				statusMessage = "❌ Failed to register Service Worker layer.";
				console.error(err);
			}
		} else {
			statusMessage = "❌ Browser does not support Web Push notifications.";
		}
	});

	async function handleSubscribe() {
		if (!city.trim() || !barangay.trim()) {
			statusMessage = "⚠️ Pakisulat po ang Lungsod at Barangay.";
			return;
		}

		isButtonDisabled = true;
		statusMessage = "Hinihingi ang pahintulot sa browser...";

		try {
			// Step A: Fetch the VAPID public key string from the Go backend
			const keyResponse = await fetch(`${BACKEND_URL}/api/vapid-public-key`);
			const { publicKey } = await keyResponse.json();
			const convertedPublicKey = urlBase64ToUint8Array(publicKey);

			// Step B: Trigger the browser's native permission request window
			const subscription = await registration.pushManager.subscribe({
				userVisibleOnly: true,
				applicationServerKey: convertedPublicKey,
			});

			statusMessage = "Isinesave ang subscription...";

			// Step C: Post credentials to your Go API server
			const response = await fetch(`${BACKEND_URL}/api/subscribe`, {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					city: city.trim(),
					barangay: barangay.trim(),
					subscription: subscription,
				}),
			});

			const result = await response.json();

			if (response.ok) {
				statusMessage =
					"✅ Naka-subscribe ka na! Makakatanggap ka na ng abiso.";
			} else {
				statusMessage = `❌ Error: ${result.error || "Failed to save subscription"}`;
				isButtonDisabled = false;
			}
		} catch (err) {
			statusMessage = "❌ Tinanggihan ang permission o nagka-error.";
			console.error(err);
			isButtonDisabled = false;
		}
	}
</script>

<main class="alert-card">
	<h2>Abiso Utility Alerts</h2>
	<p>
		Mag-subscribe para makatanggap ng real-time notifications kapag may class
		suspensions o utility interruptions.
	</p>

	<div class="form-group">
		<label for="city">Lungsod / Bayan (City):</label>
		<input
			type="text"
			id="city"
			bind:value={city}
			placeholder="Hal. Valenzuela"
		/>
	</div>

	<div class="form-group">
		<label for="barangay">Barangay:</label>
		<input
			type="text"
			id="barangay"
			bind:value={barangay}
			placeholder="Hal. Karuhatan"
		/>
	</div>

	<button onclick={handleSubscribe} disabled={isButtonDisabled}>
		{isButtonDisabled && !city ? "Loading system..." : "I-activate ang Alerts"}
	</button>

	{#if statusMessage}
		<div class="status">{statusMessage}</div>
	{/if}
</main>

<style>
	.alert-card {
		max-width: 400px;
		margin: 40px auto;
		padding: 20px;
		font-family: system-ui, sans-serif;
	}
	.form-group {
		margin-bottom: 15px;
	}
	label {
		display: block;
		margin-bottom: 5px;
		font-weight: bold;
	}
	input {
		width: 100%;
		padding: 8px;
		box-sizing: border-box;
		border: 1px solid #ccc;
		border-radius: 4px;
	}
	button {
		width: 100%;
		padding: 10px;
		background: #007acc;
		color: white;
		border: none;
		border-radius: 4px;
		font-size: 16px;
		cursor: pointer;
	}
	button:disabled {
		background: #ccc;
		cursor: not-allowed;
	}
	.status {
		margin-top: 15px;
		font-weight: bold;
	}
</style>
