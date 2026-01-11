package domain

type ConversationType string

const (
	ConversationTypeDM    ConversationType = "DM"
	ConversationTypeGroup ConversationType = "GROUP"
)

type ParticipantRole string

const (
	ParticipantRoleOwner  ParticipantRole = "OWNER"
	ParticipantRoleAdmin  ParticipantRole = "ADMIN"
	ParticipantRoleMember ParticipantRole = "MEMBER"
)

type MessageType string

const (
	MessageTypeText     MessageType = "TEXT"
	MessageTypeImage    MessageType = "IMAGE"
	MessageTypeVideo    MessageType = "VIDEO"
	MessageTypeAudio    MessageType = "AUDIO"
	MessageTypeFile     MessageType = "FILE"
	MessageTypeLocation MessageType = "LOCATION"
	MessageTypeContact  MessageType = "CONTACT"
	MessageTypeSystem   MessageType = "SYSTEM"
	MessageTypeSticker  MessageType = "STICKER"
	MessageTypeGif      MessageType = "GIF"
)

type DeliveryStatus string

const (
	DeliveryStatusPending   DeliveryStatus = "PENDING"
	DeliveryStatusSent      DeliveryStatus = "SENT"
	DeliveryStatusDelivered DeliveryStatus = "DELIVERED"
	DeliveryStatusRead      DeliveryStatus = "READ"
)

type PrivacySetting string

const (
	PrivacySettingEveryone PrivacySetting = "EVERYONE"
	PrivacySettingContacts PrivacySetting = "CONTACTS"
	PrivacySettingNobody   PrivacySetting = "NOBODY"
)

type ThemeMode string

const (
	ThemeModeSystem ThemeMode = "SYSTEM"
	ThemeModeLight  ThemeMode = "LIGHT"
	ThemeModeDark   ThemeMode = "DARK"
)

type LanguageCode string

const (
	LanguageCodeEn LanguageCode = "en"
	LanguageCodeEs LanguageCode = "es"
	LanguageCodeFr LanguageCode = "fr"
	LanguageCodeDe LanguageCode = "de"
	LanguageCodePt LanguageCode = "pt"
	LanguageCodeRu LanguageCode = "ru"
	LanguageCodeHi LanguageCode = "hi"
	LanguageCodeZh LanguageCode = "zh"
)
