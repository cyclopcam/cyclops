<template>
	<div class="root">
		<div class="topText">
			The following table describes the 3 different alert levels, depending on
			whether the system is armed or unarmed:</div>
		<table>
			<tr>
				<td></td>
				<td class="gr"></td>
				<td colspan="2" class="gr gt small">System Activation
				</td>
			</tr>
			<tr>
				<td></td>
				<td class="gr"></td>
				<td class="gr gt b">Armed</td>
				<td class="gr gt b">Unarmed</td>
			</tr>
			<tr class="">
				<td rowspan="3" class="levelBG gr gl gt gb small">Alert<br /> Level</td>
				<td class="gr gt gb b alarmBG">Alarm</td>
				<td class="gr gt gb raiseAlarm">Raise Alarm</td>
				<td class="gr gt gb">Record Video</td>
			</tr>
			<tr class="">
				<td class="gr gb b recordBG">Record</td>
				<td class="gr gb">Record Video</td>
				<td class="gr gb">Record Video</td>
			</tr>
			<tr class="">
				<td class="gr gb b ignoreBG">Ignore</td>
				<td class="gr gb">Do Nothing</td>
				<td class="gr gb">Do Nothing</td>
			</tr>
		</table>
	</div>
</template>
	
<style lang="scss" scoped>
@import '@/assets/vars.scss';
@import './tag.scss';

table {
	border-spacing: 0;
}

td {
	padding: 8px;

	@media (max-width: $mobileCutoff) {
		padding: 6px;
	}

	text-align: center;
}

.root {
	padding: 20px;

	@media (max-width: $mobileCutoff) {
		padding: 10px 10px;
	}
}

.topText {
	width: 330px;

	@media (max-width: $mobileCutoff) {
		width: 330px;
	}

	margin-bottom: 30px;
}

.small {
	font-size: 14px;
}

.b {
	font-weight: bold;
}

.levelBG {
	background-color: #fff;
}

.raiseAlarm {
	//text-decoration: underline;
	//text-decoration-color: #d00;
	//text-decoration-thickness: 3px;
	//text-underline-offset: 3px;
	color: #a00;
}

// gr = grid right
// gl = grid left
// etc
.gr {
	border-right: solid 1px #333;
}

.gl {
	border-left: solid 1px #333;
}

.gt {
	border-top: solid 1px #333;
}

.gb {
	border-bottom: solid 1px #333;
}
</style>
	