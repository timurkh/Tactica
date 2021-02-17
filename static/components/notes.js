const NoteDialog = {
	props: {
		windowId: String, 
		title: String,
		note:{
			type: Object,
			default: () => ({})
		},
	},
	emits: ["submit-form"],
    template:  `
	<div class="modal fade" :id="windowId" tabindex="-1" role="dialog">
			<div class="modal-dialog" role="document">
				<div class="modal-content">
					<div class="modal-header">
						<h5 class="modal-title">{{title}}</h5>
						<button type="button" class="close" data-dismiss="modal" aria-label="Close">
							<span aria-hidden="true">&times;</span>
						</button>
					</div>
					<form>
						<div class="modal-body">
							<div class="form-group">
								<label for="noteTitle">Title</label>
								<input type="text" id="noteTitle" class="form-control" v-model="note.title">
							</div>
							<div class="form-group">
								<label for="noteText">Note</label>
								<textarea id="noteText" class="form-control" v-model="note.text"></textarea>
							</div>
						</div>
						<div class="modal-footer">
							<button type="submit" class="btn btn-primary" v-on:click="$emit('submit-form', note)" data-dismiss="modal">Add</button>
						</div>
					</form>
				</div>
			</div>
		</div>

`
};
