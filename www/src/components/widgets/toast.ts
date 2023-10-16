export function showToast(text: string) {
	let screen = document.createElement('div');
	screen.style.position = 'fixed';
	screen.style.width = '100%';
	screen.style.height = '100%';
	screen.style.left = '0px';
	screen.style.top = '0px';
	//screen.style.pointerEvents = 'none';
	screen.style.display = 'flex';
	screen.style.flexDirection = 'column';
	screen.style.alignItems = 'center';
	screen.style.padding = '40px';
	screen.style.boxSizing = 'border-box';
	screen.style.backdropFilter = 'blur(1px)';
	screen.style.backgroundColor = 'rgba(0, 0, 0, 0.2)';

	let container = document.createElement('div');
	container.style.borderRadius = '5px';
	container.style.border = 'solid 2px #333';
	container.style.backgroundColor = '#fff';
	container.style.padding = '12px 15px';
	container.style.boxShadow = '5px 5px 25px rgba(0, 0, 0, 0.7)';
	container.style.fontWeight = 'bold';
	container.style.lineHeight = '1.8em';
	container.style.textAlign = 'center';
	container.innerText = text;
	screen.appendChild(container);

	document.body.appendChild(screen);

	setTimeout(() => document.body.removeChild(screen), 2000);
}