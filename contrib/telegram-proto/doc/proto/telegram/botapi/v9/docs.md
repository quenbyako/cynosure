# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [message.proto](#message-proto)
    - [Chat](#telegram-botapi-v9-Chat)
    - [Message](#telegram-botapi-v9-Message)
    - [User](#telegram-botapi-v9-User)
  
- [api_service.proto](#api_service-proto)
    - [SendMessageRequest](#telegram-botapi-v9-SendMessageRequest)
  
    - [TelegramService](#telegram-botapi-v9-TelegramService)
  
- [webhook_service.proto](#webhook_service-proto)
    - [Update](#telegram-botapi-v9-Update)
  
    - [WebhookService](#telegram-botapi-v9-WebhookService)
  
- [Scalar Value Types](#scalar-value-types)



<a name="message-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## message.proto



<a name="telegram-botapi-v9-Chat"></a>

### Chat



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [int64](#int64) |  |  |
| type | [string](#string) |  |  |
| title | [string](#string) | optional |  |
| username | [string](#string) | optional |  |
| first_name | [string](#string) | optional |  |
| last_name | [string](#string) | optional |  |
| is_forum | [bool](#bool) |  |  |
| is_direct_message | [bool](#bool) |  |  |






<a name="telegram-botapi-v9-Message"></a>

### Message



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| message_id | [int64](#int64) |  |  |
| message_thread_id | [int64](#int64) |  |  |
| from | [User](#telegram-botapi-v9-User) | optional | direct_message_topic |
| sender_chat | [Chat](#telegram-botapi-v9-Chat) | optional |  |
| sender_boost_count | [int64](#int64) | optional |  |
| sender_business_bot | [User](#telegram-botapi-v9-User) | optional |  |
| date | [int64](#int64) |  |  |
| business_connection_id | [string](#string) | optional |  |
| chat | [Chat](#telegram-botapi-v9-Chat) |  |  |
| is_topic_message | [bool](#bool) |  | forward_origin |
| is_automatic_forward | [bool](#bool) |  |  |
| reply_to_message | [Message](#telegram-botapi-v9-Message) | optional |  |
| reply_to_checklist_task_id | [int64](#int64) | optional | external_reply quote reply_to_story |
| via_bot | [User](#telegram-botapi-v9-User) | optional |  |
| edit_date | [int64](#int64) | optional |  |
| has_protected_content | [bool](#bool) |  |  |
| is_from_online | [bool](#bool) |  |  |
| is_paid_post | [bool](#bool) |  |  |
| media_group_id | [string](#string) | optional |  |
| author_signature | [string](#string) | optional |  |
| paid_star_count | [int64](#int64) | optional |  |
| text | [string](#string) | optional |  |
| effect_id | [string](#string) | optional | repeated MessageEntity entities = 28 [json_name = &#34;entities&#34;]; optional LinkPreviewOptions link_preview_options = 29 [json_name = &#34;link_preview_options&#34;]; optional SuggestedPostInfo suggested_post_info = 30 [json_name = &#34;suggested_post_info&#34;]; |
| caption | [string](#string) | optional | optional Animation animation 	 = 33 [json_name = &#34;animation&#34;]; optional Audio audio 	 = 34 [json_name = &#34;audio&#34;]; optional Document document 	 = 35 [json_name = &#34;document&#34;]; optional PaidMediaInfo paid_media 	 = 36 [json_name = &#34;paid_media&#34;]; repeated PhotoSize photo = 37 [json_name = &#34;photo&#34;]; optional Sticker sticker 	 = 38 [json_name = &#34;sticker&#34;]; optional Story story 	 = 39 [json_name = &#34;story&#34;]; optional Video video 	 = 40 [json_name = &#34;video&#34;]; optional VideoNote video_note 	 = 41 [json_name = &#34;video_note&#34;]; optional Voice voice 	 = 42 [json_name = &#34;voice&#34;]; |
| show_caption_above_media | [bool](#bool) |  | repeated MessageEntity caption_entities 	 = 44 [json_name = &#34;caption_entities&#34;]; |
| has_media_spoiler | [bool](#bool) |  |  |
| new_chat_members | [User](#telegram-botapi-v9-User) | repeated | optional Checklist checklist = 47 [json_name = &#34;checklist&#34;]; optional Contact contact = 48 [json_name = &#34;contact&#34;]; optional Dice dice = 49 [json_name = &#34;dice&#34;]; optional Game game = 50 [json_name = &#34;game&#34;]; optional Poll poll = 51 [json_name = &#34;poll&#34;]; optional Venue venue = 52 [json_name = &#34;venue&#34;]; optional Location location = 53 [json_name = &#34;location&#34;]; |
| left_chat_member | [User](#telegram-botapi-v9-User) | optional |  |
| new_chat_title | [string](#string) | optional |  |
| delete_chat_photo | [bool](#bool) |  | repeated PhotoSize new_chat_photo = 57 [json_name = &#34;new_chat_photo&#34;]; |
| group_chat_created | [bool](#bool) |  |  |
| supergroup_chat_created | [bool](#bool) |  |  |
| channel_chat_created | [bool](#bool) |  |  |
| migrate_to_chat_id | [int64](#int64) | optional | optional MessageAutoDeleteTimerChanged message_auto_delete_timer_changed = 62 [json_name = &#34;message_auto_delete_timer_changed&#34;]; |
| migrate_from_chat_id | [int64](#int64) | optional |  |
| connected_website | [string](#string) | optional | optional MaybeInaccessibleMessage pinned_message = 65 [json_name = &#34;pinned_message&#34;]; optional Invoice invoice = 66 [json_name = &#34;invoice&#34;]; optional SuccessfulPayment successful_payment = 67 [json_name = &#34;successful_payment&#34;]; optional RefundedPayment refunded_payment = 68 [json_name = &#34;refunded_payment&#34;]; optional UsersShared users_shared = 69 [json_name = &#34;users_shared&#34;]; optional ChatShared chat_shared = 70 [json_name = &#34;chat_shared&#34;]; optional GiftInfo gift = 71 [json_name = &#34;gift&#34;]; optional UniqueGiftInfo unique_gift = 72 [json_name = &#34;unique_gift&#34;];

optional WriteAccessAllowed write_access_allowed = 74 [json_name = &#34;write_access_allowed&#34;]; optional PassportData passport_data = 75 [json_name = &#34;passport_data&#34;]; optional ProximityAlertTriggered proximity_alert_triggered = 76 [json_name = &#34;proximity_alert_triggered&#34;]; optional ChatBoostAdded boost_added = 77 [json_name = &#34;boost_added&#34;]; optional ChatBackground chat_background_set = 78 [json_name = &#34;chat_background_set&#34;]; optional ChecklistTasksDone checklist_tasks_done = 79 [json_name = &#34;checklist_tasks_done&#34;]; optional ChecklistTasksAdded checklist_tasks_added = 80 [json_name = &#34;checklist_tasks_added&#34;]; optional DirectMessagePriceChanged direct_message_price_changed = 81 [json_name = &#34;direct_message_price_changed&#34;]; optional ForumTopicCreated forum_topic_created = 82 [json_name = &#34;forum_topic_created&#34;]; optional ForumTopicEdited forum_topic_edited = 83 [json_name = &#34;forum_topic_edited&#34;]; optional ForumTopicClosed forum_topic_closed = 84 [json_name = &#34;forum_topic_closed&#34;]; optional ForumTopicReopened forum_topic_reopened = 85 [json_name = &#34;forum_topic_reopened&#34;]; optional GeneralForumTopicHidden general_forum_topic_hidden = 86 [json_name = &#34;general_forum_topic_hidden&#34;]; optional GeneralForumTopicUnhidden general_forum_topic_unhidden = 87 [json_name = &#34;general_forum_topic_unhidden&#34;]; optional GiveawayCreated giveaway_created = 88 [json_name = &#34;giveaway_created&#34;]; optional Giveaway giveaway = 89 [json_name = &#34;giveaway&#34;]; optional GiveawayWinners giveaway_winners = 90 [json_name = &#34;giveaway_winners&#34;]; optional GiveawayCompleted giveaway_completed = 91 [json_name = &#34;giveaway_completed&#34;]; optional PaidMessagePriceChanged paid_message_price_changed = 92 [json_name = &#34;paid_message_price_changed&#34;]; optional SuggestedPostApproved suggested_post_approved = 93 [json_name = &#34;suggested_post_approved&#34;]; optional SuggestedPostApprovalFailed suggested_post_approval_failed = 94 [json_name = &#34;suggested_post_approval_failed&#34;]; optional SuggestedPostDeclined suggested_post_declined = 95 [json_name = &#34;suggested_post_declined&#34;]; optional SuggestedPostPaid suggested_post_paid = 96 [json_name = &#34;suggested_post_paid&#34;]; optional SuggestedPostRefunded suggested_post_refunded = 97 [json_name = &#34;suggested_post_refunded&#34;]; optional VideoChatScheduled video_chat_scheduled = 98 [json_name = &#34;video_chat_scheduled&#34;]; optional VideoChatStarted video_chat_started = 99 [json_name = &#34;video_chat_started&#34;]; optional VideoChatEnded video_chat_ended = 100 [json_name = &#34;video_chat_ended&#34;]; optional VideoChatParticipantsInvited video_chat_participants_invited = 101 [json_name = &#34;video_chat_participants_invited&#34;]; optional WebAppData web_app_data = 102 [json_name = &#34;web_app_data&#34;]; optional InlineKeyboardMarkup reply_markup = 103 [json_name = &#34;reply_markup&#34;]; |






<a name="telegram-botapi-v9-User"></a>

### User



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [int64](#int64) |  |  |
| is_bot | [bool](#bool) |  |  |
| first_name | [string](#string) |  |  |
| last_name | [string](#string) | optional |  |
| username | [string](#string) | optional |  |
| language_code | [string](#string) | optional |  |
| is_premium | [bool](#bool) |  |  |
| added_to_attachment_menu | [bool](#bool) |  |  |
| can_join_groups | [bool](#bool) |  |  |
| can_read_all_group_messages | [bool](#bool) |  |  |
| supports_inline_queries | [bool](#bool) |  |  |
| can_connect_to_business | [bool](#bool) |  |  |
| has_main_web_app | [bool](#bool) |  |  |





 

 

 

 



<a name="api_service-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## api_service.proto



<a name="telegram-botapi-v9-SendMessageRequest"></a>

### SendMessageRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| bot_token | [string](#string) |  |  |
| chat_id | [int64](#int64) |  |  |
| text | [string](#string) |  |  |





 

 

 


<a name="telegram-botapi-v9-TelegramService"></a>

### TelegramService


| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| SendMessage | [SendMessageRequest](#telegram-botapi-v9-SendMessageRequest) | [Message](#telegram-botapi-v9-Message) |  |

 



<a name="webhook_service-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## webhook_service.proto



<a name="telegram-botapi-v9-Update"></a>

### Update



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| update_id | [int64](#int64) |  |  |
| message | [Message](#telegram-botapi-v9-Message) |  |  |





 

 

 


<a name="telegram-botapi-v9-WebhookService"></a>

### WebhookService


| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| SendUpdate | [Update](#telegram-botapi-v9-Update) | [.google.protobuf.Empty](#google-protobuf-Empty) |  |

 



## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |

