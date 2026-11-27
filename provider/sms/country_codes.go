package sms

// ITU-T E.164 已分配的地理国家/地区区号表（静态表，0 新依赖，不引入 libphonenumber）。
//
// 为什么需要它：规格 §6.2 的区号正则 ^[1-9]\d{0,3}$ 允许任意 1-4 位切分，
// 只校验两段各自合法会让同一串 E164 产出多个 (Country, Local) 键——
// +8613900001111 可切成 (86,13900001111)/(8,61390001111)/(861,3900001111)/(8613,900001111)。
// 任务 6-8 把号码日限、重发冷却与查重都建在 (phone_country, phone) 上，
// 换切分即可乘出多份预算；Region() 也会把大陆号误判成 international，
// 绕过规格 §3.3 规则 3「Twilio + 86 明确拒发」。
// ITU 区号是前缀码（没有任何一个是另一个的真前缀），表一上切分即唯一，
// 因此 Region() 无需改动。该不变量由 TestCountryCodesPrefixFree 穷举锁住。
//
// 来源与核对日期：2026-11-14 核对，三源交叉（均为可核验的权威清单，非凭记忆默写）。
//  1. ITU-T E.164D《List of Recommendation ITU-T E.164 assigned country codes》
//     Position on 15 December 2016，Annex to ITU Operational Bulletin No. 1114-E，20 页 PDF：
//     https://www.itu.int/dms_pub/itu-t/opb/sp/T-SP-E.164D-2016-PDF-E.pdf
//     数字序清单 334 行，去掉 90 行 Spare code/Reserved 后 217 个不同码，
//     再去掉其中 11 个非地理号段 → 206 个。
//  2. ITU 官方 National Numbering Plans 在线清单（实时）：
//     https://www.itu.int/oth/T0202.aspx?parent=T0202
//     245 个 option → 210 条「码 (名称)」→ 207 个不同码（含 870 Inmarsat），
//     去掉 870 → 206 个地理码。另有 35 条子号段 option（NANP 的 1 242 等 27 条、
//     卫星的 881 6/882 16 等 8 条），它们的国家码仍是 1/881/882，不单独收录。
//  3. Google libphonenumber PhoneNumberMetadata.xml（master，实时）：
//     https://raw.githubusercontent.com/google/libphonenumber/master/resources/PhoneNumberMetadata.xml
//     254 个 territory → 215 个不同 countryCode，其中 9 个 id="001" 为非地理
//     （800/808/870/878/881/882/883/888/979）→ 206 个地理码。
//
// 差集：来源 2 与来源 3 的地理码集合完全相同（双向差集为空）。
// 来源 1（2016 快照）与来源 2/3 的差集只有两项：
//   - 379 Vatican City State：PDF 注 f 写明「Reserved for future use」，梵蒂冈实际
//     使用 39（来源 3 把 VA 映射到 39），来源 2 也不列 379 → 不收录。
//   - 970 Palestine, State of：2016 PDF 里仍是「Reserved l」（注 l 为巴勒斯坦保留），
//     来源 2/3 均已启用 → 收录。
//
// 最终收录 206 个地理国家/地区码（含 852/853/886）。
//
// 排除的非地理号段（短信是人对人投递，这些号段不可达）：
// 388 国家组共享码、800 国际免费电话、808 国际共享成本、870 Inmarsat 卫星、
// 878 UPT、879 国内非商业用途保留、881 GMSS 卫星、882/883 国际网络、
// 888 灾难救济 TDR、979 国际高费率业务、991 新业务试验。
// 由 TestCountryCodesExcludeNonGeographic 锁住。
//
// 维护：区号变更时按上述来源重新核对，并始终保持前缀无关（跑护栏测试）。
var countryCodes = map[string]struct{}{
	"1":   {}, // Canada / United States（NANP，含加勒比与太平洋属地）
	"7":   {}, // Kazakhstan / Russian Federation
	"20":  {}, // Egypt
	"27":  {}, // South Africa
	"30":  {}, // Greece
	"31":  {}, // Netherlands
	"32":  {}, // Belgium
	"33":  {}, // France
	"34":  {}, // Spain
	"36":  {}, // Hungary
	"39":  {}, // Italy
	"40":  {}, // Romania
	"41":  {}, // Switzerland
	"43":  {}, // Austria
	"44":  {}, // United Kingdom
	"45":  {}, // Denmark
	"46":  {}, // Sweden
	"47":  {}, // Norway
	"48":  {}, // Poland
	"49":  {}, // Germany
	"51":  {}, // Peru
	"52":  {}, // Mexico
	"53":  {}, // Cuba
	"54":  {}, // Argentina
	"55":  {}, // Brazil
	"56":  {}, // Chile
	"57":  {}, // Colombia
	"58":  {}, // Venezuela
	"60":  {}, // Malaysia
	"61":  {}, // Australia
	"62":  {}, // Indonesia
	"63":  {}, // Philippines
	"64":  {}, // New Zealand
	"65":  {}, // Singapore
	"66":  {}, // Thailand
	"81":  {}, // Japan
	"82":  {}, // Korea (Rep. of)
	"84":  {}, // Viet Nam
	"86":  {}, // China
	"90":  {}, // Türkiye
	"91":  {}, // India
	"92":  {}, // Pakistan
	"93":  {}, // Afghanistan
	"94":  {}, // Sri Lanka
	"95":  {}, // Myanmar
	"98":  {}, // Iran (Islamic Republic of)
	"211": {}, // South Sudan
	"212": {}, // Morocco
	"213": {}, // Algeria
	"216": {}, // Tunisia
	"218": {}, // Libya
	"220": {}, // Gambia
	"221": {}, // Senegal
	"222": {}, // Mauritania
	"223": {}, // Mali
	"224": {}, // Guinea
	"225": {}, // Côte d'Ivoire
	"226": {}, // Burkina Faso
	"227": {}, // Niger
	"228": {}, // Togo
	"229": {}, // Benin
	"230": {}, // Mauritius
	"231": {}, // Liberia
	"232": {}, // Sierra Leone
	"233": {}, // Ghana
	"234": {}, // Nigeria
	"235": {}, // Chad
	"236": {}, // Central African Rep.
	"237": {}, // Cameroon
	"238": {}, // Cabo Verde
	"239": {}, // Sao Tome and Principe
	"240": {}, // Equatorial Guinea
	"241": {}, // Gabon
	"242": {}, // Congo (Rep. of the)
	"243": {}, // Dem. Rep. of the Congo
	"244": {}, // Angola
	"245": {}, // Guinea-Bissau
	"246": {}, // Diego Garcia
	"247": {}, // Ascension
	"248": {}, // Seychelles
	"249": {}, // Sudan
	"250": {}, // Rwanda
	"251": {}, // Ethiopia
	"252": {}, // Somalia
	"253": {}, // Djibouti
	"254": {}, // Kenya
	"255": {}, // Tanzania
	"256": {}, // Uganda
	"257": {}, // Burundi
	"258": {}, // Mozambique
	"260": {}, // Zambia
	"261": {}, // Madagascar
	"262": {}, // French Departments and Territories in the Indian Ocean
	"263": {}, // Zimbabwe
	"264": {}, // Namibia
	"265": {}, // Malawi
	"266": {}, // Lesotho
	"267": {}, // Botswana
	"268": {}, // Eswatini
	"269": {}, // Comoros
	"290": {}, // Saint Helena and Tristan da Cunha
	"291": {}, // Eritrea
	"297": {}, // Aruba
	"298": {}, // Faroe Islands
	"299": {}, // Greenland
	"350": {}, // Gibraltar
	"351": {}, // Portugal
	"352": {}, // Luxembourg
	"353": {}, // Ireland
	"354": {}, // Iceland
	"355": {}, // Albania
	"356": {}, // Malta
	"357": {}, // Cyprus
	"358": {}, // Finland
	"359": {}, // Bulgaria
	"370": {}, // Lithuania
	"371": {}, // Latvia
	"372": {}, // Estonia
	"373": {}, // Moldova
	"374": {}, // Armenia
	"375": {}, // Belarus
	"376": {}, // Andorra
	"377": {}, // Monaco
	"378": {}, // San Marino
	"380": {}, // Ukraine
	"381": {}, // Serbia
	"382": {}, // Montenegro
	"383": {}, // Kosovo*
	"385": {}, // Croatia
	"386": {}, // Slovenia
	"387": {}, // Bosnia and Herzegovina
	"389": {}, // North Macedonia
	"420": {}, // Czech Rep.
	"421": {}, // Slovakia
	"423": {}, // Liechtenstein
	"500": {}, // Falkland Islands (Malvinas)
	"501": {}, // Belize
	"502": {}, // Guatemala
	"503": {}, // El Salvador
	"504": {}, // Honduras
	"505": {}, // Nicaragua
	"506": {}, // Costa Rica
	"507": {}, // Panama
	"508": {}, // Saint Pierre and Miquelon
	"509": {}, // Haiti
	"590": {}, // Guadeloupe
	"591": {}, // Bolivia (Plurinational State of)
	"592": {}, // Guyana
	"593": {}, // Ecuador
	"594": {}, // French Guiana
	"595": {}, // Paraguay
	"596": {}, // Martinique
	"597": {}, // Suriname
	"598": {}, // Uruguay
	"599": {}, // Bonaire, Sint Eustatius and Saba / Curaçao
	"670": {}, // Timor-Leste
	"672": {}, // Norfolk Island
	"673": {}, // Brunei Darussalam
	"674": {}, // Nauru
	"675": {}, // Papua New Guinea
	"676": {}, // Tonga
	"677": {}, // Solomon Islands
	"678": {}, // Vanuatu
	"679": {}, // Fiji
	"680": {}, // Palau
	"681": {}, // Wallis and Futuna
	"682": {}, // Cook Islands
	"683": {}, // Niue
	"685": {}, // Samoa
	"686": {}, // Kiribati
	"687": {}, // New Caledonia
	"688": {}, // Tuvalu
	"689": {}, // French Polynesia
	"690": {}, // Tokelau
	"691": {}, // Micronesia
	"692": {}, // Marshall Islands
	"850": {}, // Dem. People's Rep. of Korea
	"852": {}, // Hong Kong, China
	"853": {}, // Macao, China
	"855": {}, // Cambodia
	"856": {}, // Lao P.D.R.
	"880": {}, // Bangladesh
	"886": {}, // Taiwan, China
	"960": {}, // Maldives
	"961": {}, // Lebanon
	"962": {}, // Jordan
	"963": {}, // Syrian Arab Republic
	"964": {}, // Iraq
	"965": {}, // Kuwait
	"966": {}, // Saudi Arabia
	"967": {}, // Yemen
	"968": {}, // Oman
	"970": {}, // Palestine, State of
	"971": {}, // United Arab Emirates
	"972": {}, // Israel
	"973": {}, // Bahrain
	"974": {}, // Qatar
	"975": {}, // Bhutan
	"976": {}, // Mongolia
	"977": {}, // Nepal
	"992": {}, // Tajikistan
	"993": {}, // Turkmenistan
	"994": {}, // Azerbaijan
	"995": {}, // Georgia
	"996": {}, // Kyrgyzstan
	"998": {}, // Uzbekistan
}
