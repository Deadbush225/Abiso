self.addEventListener("push", function (event) {
	if (!event.data) {
		console.log("Push event received with no payload data.");
		return;
	}

	try {
		// Extract the notification JSON structure dispatched from your Go backend
		const data = event.data.json();

		const title = data.title || "ABISO ADVISORY";
		const options = {
			body:
				data.body || "Mayroon kang bagong alert tungkol sa inyong komunidad.",
			icon: "/icon.png", // Path to your application brand icon asset
			badge: "/badge.png", // Path to small monochrome notification bar tray asset
			vibrate: [200, 100, 200], // Vibration pattern sequence for supported mobile platforms
			data: {
				dateOfArrival: Date.now(),
				primaryKey: "1",
			},
		};

		event.waitUntil(self.registration.showNotification(title, options));
	} catch (err) {
		console.error("Error rendering incoming push event structure: ", err);
	}
});

// Optional action click pipeline routing target behaviors when a user taps the notification card
self.addEventListener("notificationclick", function (event) {
	event.notification.close();
	event.waitUntil(
		clients.openWindow("/"), // Routes target engagement directly back to your service dashboard root
	);
});
