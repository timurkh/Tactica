const { createApp } = Vue

const app = createApp( {
	delimiters: ['[[', ']]'],
	data(){
		return {
			loading:true,
			error_message:"",
			squadId:squadId,
			squad_members:[],
			squad_owner:null,
			statusToSet:0,
			changeMember_index: -1,
			changeMember: [],
			replicant: [],
			tags:[],
			tagToSet:{},
			tagToSetValue:"",
		};
	},
	created:function() {
		axios.all([
			axios.get(`/methods/squads/${squadId}/members`),
			axios.get(`/methods/squads/${squadId}/tags`),
		])
		.then(axios.spread((members, tags) => {
			this.squad_members = members.data['Members']; 
			this.squad_owner = members.data['Owner'];
			this.tags = tags.data;
			this.loading = false;
		}))
		.catch(error => {
			this.error_message = "Failed to retrieve squad members and tags: " + this.getAxiosErrorMessage(error);
			this.loading = false;
		});
	},
	methods: {
		changeStatus:function(member, index) {
			this.changeMember_index = index;
			this.changeMember = member;
			this.statusToSet = member.status;
			$('#changeMemberStatusModal').modal('show')
		},
		tagMember:function(member, index) {
			this.changeMember_index = index;
			this.changeMember = member;
			this.tagToSet = this.tags[0];
			$('#addTagModal').modal('show')
		},
		setMemberTag:function() {
			let tag = new Object();
			tag.Name = this.tagToSet.name;
			if(this.getTagHasValues(this.tagToSet))
				tag.Value = this.tagToSetValue;
			axios({
				method: 'POST',
				url: `/methods/squads/${squadId}/members/${this.changeMember.id}/tags`,
				data: { Tag: tag}, 
				headers: { "X-CSRF-Token": csrfToken },
			})
			.then( res => {
				this.error_message = "";
				this.squad_members[this.changeMember_index].tags = res.data.tags;
			})
			.catch(err => {
				this.error_message = "Error while tagging member: " + this.getAxiosErrorMessage(err);
			});
		},
		deleteMemberTag:function(member, tag, tagIndex) {
			axios({
				method: 'DELETE',
				url: `/methods/squads/${squadId}/members/${member.id}/tags/${tag}`,
				headers: { "X-CSRF-Token": csrfToken },
			})
			.then( res => {
				this.error_message = "";
				member.tags = res.data.tags;
			})
			.catch(err => {
				this.error_message = "Error while updating member tags: " + this.getAxiosErrorMessage(err);
			});
		},
		setMemberStatus:function() {
			axios({
				method: 'PATCH',
				url: `/methods/squads/${squadId}/members/${this.changeMember.id}`,
				data: { Status: this.statusToSet, },
				headers: { "X-CSRF-Token": csrfToken },
			})
			.then( res => {
				this.error_message = "";
				this.squad_members[this.changeMember_index].status = this.statusToSet;
			})
			.catch(err => {
				this.error_message = "Error while changing member status: " + this.getAxiosErrorMessage(err);
			});
		},
		removeMember:function(userId, index) {
			index = index;
			axios({
				method: 'DELETE',
				url: `/methods/squads/${squadId}/members/${userId}`,
				headers: { "X-CSRF-Token": csrfToken },
			})
			.then( res => {
				this.error_message = "";
				this.squad_members.splice(index, 1);
			})
			.catch(err => {
				this.error_message = `Error while removing user ${userId} from squad:` + this.getAxiosErrorMessage(err);
			});
		},
		addMember:function() {
			axios({
				method: 'POST',
				url: `/methods/squads/${squadId}/members`,
				data: {
					displayName : this.replicant.displayName,
					email : this.replicant.email,
					phoneNumber : this.replicant.phoneNumber,
					replicant: true,
				},
				headers: { "X-CSRF-Token": csrfToken },
			})
			.then( res => {
				this.error_message = "";
				this.replicant.id = res.data.ReplicantId;
				this.replicant.status = 1; //member
				this.replicant.replicant = true;
				this.squad_members.push(Object.assign({}, this.replicant));
			})
			.catch(err => {
				this.error_message = "Error while adding squad member: " + this.getAxiosErrorMessage(err);
			});
		}
	},
	computed: {
		newTagValue: {
			get: function() {
				if (this.tagToSetValue.length == 0) {
					this.tagToSetValue = Object.keys(this.tagToSet.values)[0];
				}

				return this.tagToSetValue;
			},
			set: function (value) {
				this.tagToSetValue = value;
			}
		},
	}, 
	mixins: [globalMixin],
}).mount("#app");
