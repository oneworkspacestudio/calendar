// Generated from Calendar.aui. Do not edit.
import { Alert, Button, Card, Checkbox, Divider, Grid, Heading, Icon, Image, Input, Link, Page, Row, Section, Select, Spacer, Stack, Text } from "@/components/ui"

export function CalendarScreen({ data, onAction }: { data: any; onAction: (name: string) => void }) {
  return (
    <Page>
      {data.configLoaded ? (
        <Section role="app-bootstrap">
          {data.setup ? (
            <Card role="setup-card">
              <Image src="calendar.svg" alt="My Calendar" width={48} />
              <Heading level={1}>Set up your workspace</Heading>
              <Text>Calendar, Mail and Contacts share one private Stalwart service. You only need to set it up once.</Text>
              <Alert tone="error" text={data.error} />
              <Input field="issuer" label="MyAccounts URL" value={data.form.issuer} placeholder="https://login.example.com" />
              <Input field="clientId" label="Shared client ID" value={data.form.clientId} />
              <Input field="clientSecret" label="Client secret" type="password" value={data.form.clientSecret} />
              <Text role="hint">Register this app's callback in the shared MyAccounts client. Each app's exact callback must be allowed.</Text>
              <Text text={data.callback} />
              <Button variant="primary" onClick={() => onAction("setup-save")} disabled={data.busy}>Connect Calendar</Button>
            </Card>
          ) : (
            <Section role="app-root">
              {data.signedIn ? (
                <>
                  <Row role="header">
                    <Button icon="menu" title="Main menu" onClick={() => onAction("toggle-sidebar")} />
                    <Row role="brand">
                      <Image src={data.appIcon} alt={data.appTitle} width={36} />
                      <Text role="brand-title" text={data.appTitle} />
                    </Row>
                    {data.showCalendar && (
                      <>
                        <Button role="today" onClick={() => onAction("today")}>Today</Button>
                        <Button icon="chevron-left" title="Previous period" onClick={() => onAction("previous")} />
                        <Button icon="chevron-right" title="Next period" onClick={() => onAction("next")} />
                        <Text role="period" text={data.period} />
                      </>
                    )}
                    <Spacer />
                    {data.searching ? (
                      <>
                        <Input role="search" field="query" placeholder={data.searchPlaceholder} value={data.query} />
                        <Button icon="x" title="Close search" onClick={() => onAction("close-search")} />
                      </>
                    ) : (
                      <Button icon="search" title="Search" onClick={() => onAction("search")} />
                    )}
                    <Button icon="circle-help" title="Support" onClick={() => onAction("help")} />
                    {data.showCalendar && (
                      <Button icon="settings" title="Settings" onClick={() => onAction("settings")} />
                    )}
                    {data.showCalendar && (
                      <>
                        <Select role="view" field="view" value={data.view} options={data.viewOptions} label="Calendar view" />
                        <Row role="mode-switch">
                          <Button icon="calendar-days" title="Switch to Calendar" onClick={() => onAction("calendar-mode")} selected={data.calendarMode} />
                          <Button icon="circle-check" title="Switch to Tasks" onClick={() => onAction("tasks-mode")} selected={data.tasksMode} />
                        </Row>
                      </>
                    )}
                    <Button icon="grid-3x3" title="Workspace apps" onClick={() => onAction("apps")} />
                    <Button role="avatar" title="Your account" onClick={() => onAction("account")} text={data.initial} />
                  </Row>
                  <Row role="workspace">
                    {data.sidebar && (
                      <Stack role="sidebar">
                        {data.showCalendar && (
                          <>
                            <Button role="create" icon="plus" onClick={() => onAction("toggle-create")}>
                              Create
                              <Icon name="chevron-down" />
                            </Button>
                            {data.createMenu && (
                              <Card role="create-menu">
                                <Button onClick={() => onAction("create")}>Event</Button>
                                <Button onClick={() => onAction("create-task")}>Task</Button>
                                <Button onClick={() => onAction("create-appointment")}>Appointment schedule</Button>
                              </Card>
                            )}
                            <Row role="mini-title">
                              <Text text={data.miniTitle} />
                              <Spacer />
                              <Button icon="chevron-left" title="Previous month" onClick={() => onAction("mini-previous")} />
                              <Button icon="chevron-right" title="Next month" onClick={() => onAction("mini-next")} />
                            </Row>
                            <Grid role="mini-weekdays" cols={7}>
                              {data.weekdayLetters.map((item, i) => (
                                <Text key={i} text={item} />
                              ))}
                            </Grid>
                            <Grid role="mini-grid" cols={7}>
                              {data.miniDays.map((item, i) => (
                                <Button key={i} role="mini-date" datum={item} text={item.day} selected={item.selected} muted={item.muted} today={item.today} />
                              ))}
                            </Grid>
                          </>
                        )}
                        {data.contactsMode && (
                          <>
                            <Button role="create" icon="plus" onClick={() => onAction("create-contact")}>Create contact</Button>
                            <Text role="hint">Your contacts</Text>
                          </>
                        )}
                        <Input role="people" icon="users" field="query" placeholder="Search for people" value={data.query} />
                        {data.showCalendar && (
                          <>
                            <Button role="sidebar-row" icon="plus" onClick={() => onAction("booking")}>Booking pages</Button>
                            <Button role="sidebar-heading" onClick={() => onAction("toggle-calendars")}>
                              My calendars
                              <Icon name="chevron-up" />
                            </Button>
                            {data.showCalendars && (
                              <Section role="conditional-group">
                                {data.calendars.map((item, i) => (
                                  <Checkbox key={i} role="calendar-check" datum={item} label={item.name} checked={item.visible} color={item.color} />
                                ))}
                              </Section>
                            )}
                            <Button role="sidebar-heading" onClick={() => onAction("new-calendar")}>
                              Other calendars
                              <Icon name="plus" />
                            </Button>
                          </>
                        )}
                        {data.demo && (
                          <Text role="demo-note">Demo · sample events only</Text>
                        )}
                        <Spacer />
                        <Row role="sidebar-footer">
                          <Link href={data.workspaceHome}>One Workspace</Link>
                          <Button onClick={() => onAction("service-status")}>Service status</Button>
                        </Row>
                      </Stack>
                    )}
                    <Section role="calendar-surface">
                      {data.error && (
                        <Alert tone="error" text={data.error}><Button onClick={() => onAction("refresh")}>Retry</Button></Alert>
                      )}
                      {data.busy && (
                        <Text role="loading" text={data.syncLabel} />
                      )}
                      {data.calendarSearching ? (
                        <Section role="agenda">
                          <Heading level={2}>Search results</Heading>
                          {data.agenda.map((item, i) => (
                            <Button key={i} role="agenda-event" datum={item}>
                              <Text role="agenda-date" text={item.dayLabel} />
                              <Text role="agenda-time" text={item.timeLabel} />
                              <Icon name="circle" color={item.color} />
                              <Text text={item.title} />
                            </Button>
                          ))}
                          {data.empty && (
                            <Text role="empty">No matching events</Text>
                          )}
                        </Section>
                      ) : (
                        <>
                          {data.weekView && (
                            <>
                              <Grid role="day-headings" cols={data.columnCount}>
                                <Text role="timezone" text={data.timezoneLabel} />
                                {data.days.map((item, i) => (
                                  <Button key={i} role="day-heading" datum={item} today={item.today}>
                                    <Text role="day-label" text={item.label} />
                                    <Text role="day-number" text={item.number} />
                                  </Button>
                                ))}
                              </Grid>
                              <Grid role="all-day-row" cols={data.columnCount}>
                                <Text role="timezone">all-day</Text>
                                {data.days.map((item, i) => (
                                  <Section key={i} role="all-day-column" datum={item}>
                                    {item.allDay.map((item, i) => (
                                      <Button key={i} role="all-day-event" datum={item} color={item.color} text={item.title} />
                                    ))}
                                  </Section>
                                ))}
                              </Grid>
                              <Section role="time-scroll">
                                <Grid role="time-grid" cols={data.columnCount}>
                                  <Stack role="hour-labels">
                                    {data.hours.map((item, i) => (
                                      <Text key={i} role="hour-label" text={item} />
                                    ))}
                                  </Stack>
                                  {data.days.map((item, i) => (
                                    <Section key={i} role="day-column" datum={item}>
                                      {item.events.map((item, i) => (
                                        <Section key={i} role="event" datum={item} color={item.color}>
                                          <Button role="event-body" datum={item}>
                                            <Text role="event-title" text={item.title} />
                                            <Text role="event-time" text={item.timeLabel} />
                                          </Button>
                                          <Button role="resize-handle" datum={item} title="Resize event" />
                                        </Section>
                                      ))}
                                      {item.showNow && (
                                        <Section role="now-line" datum={item} />
                                      )}
                                    </Section>
                                  ))}
                                </Grid>
                              </Section>
                            </>
                          )}
                          {data.monthView && (
                            <>
                              <Grid role="month-weekdays" cols={data.monthColumns}>
                                {data.weekdayNames.map((item, i) => (
                                  <Text key={i} text={item} />
                                ))}
                              </Grid>
                              <Grid role="month-grid" cols={data.monthColumns}>
                                {data.monthDays.map((item, i) => (
                                  <Section key={i} role="month-cell" datum={item} muted={item.muted}>
                                    <Button role="month-number" datum={item} today={item.today} text={item.number} />
                                    {item.allEvents.map((item, i) => (
                                      <Button key={i} role="month-event" datum={item} color={item.color}>
                                        <Text role="month-time" text={item.shortTime} />
                                        <Text text={item.title} />
                                      </Button>
                                    ))}
                                  </Section>
                                ))}
                              </Grid>
                            </>
                          )}
                          {data.yearView && (
                            <Grid role="year-grid" cols={3}>
                              {data.yearMonths.map((item, i) => (
                                <Section key={i} role="year-month">
                                  <Heading level={3} text={item.title} />
                                  <Grid role="mini-weekdays" cols={7}>
                                    {data.weekdayLetters.map((item, i) => (
                                      <Text key={i} text={item} />
                                    ))}
                                  </Grid>
                                  <Grid role="mini-grid" cols={7}>
                                    {item.days.map((item, i) => (
                                      <Button key={i} role="mini-date" datum={item} text={item.day} today={item.today} muted={item.muted} />
                                    ))}
                                  </Grid>
                                </Section>
                              ))}
                            </Grid>
                          )}
                          {data.scheduleView && (
                            <Section role="agenda">
                              {data.agenda.map((item, i) => (
                                <Button key={i} role="agenda-event" datum={item}>
                                  <Text role="agenda-date" text={item.dayLabel} />
                                  <Text role="agenda-time" text={item.timeLabel} />
                                  <Icon name="circle" color={item.color} />
                                  <Text text={item.title} />
                                </Button>
                              ))}
                              {data.empty && (
                                <Text role="empty">No events to show. Create an event to get started.</Text>
                              )}
                            </Section>
                          )}
                          {data.contactsMode && (
                            <Section role="contacts">
                              <Row>
                                <Heading level={1}>Contacts</Heading>
                                <Spacer />
                                <Button variant="primary" icon="plus" onClick={() => onAction("create-contact")}>Create contact</Button>
                              </Row>
                              {data.contacts.map((item, i) => (
                                <Button key={i} role="contact-row" datum={item}>
                                  <Text role="contact-avatar" text={item.initial} />
                                  <Text text={item.name} />
                                  <Text text={item.email} />
                                  <Text text={item.phone} />
                                </Button>
                              ))}
                            </Section>
                          )}
                        </>
                      )}
                    </Section>
                    {data.showCalendar && (
                      <Stack role="right-rail">
                        <Button icon="sticky-note" title="Notes" onClick={() => onAction("notes")} />
                        <Button icon="circle-check" title="Tasks" onClick={() => onAction("tasks-mode")} />
                        <Button icon="contact-round" title="Contacts" onClick={() => onAction("contacts")} />
                        <Divider />
                        <Button icon="plus" title="Add to your workspace" onClick={() => onAction("apps")} />
                        <Spacer />
                        <Button icon="chevron-right" title="Hide side panel" onClick={() => onAction("toggle-rail")} />
                      </Stack>
                    )}
                  </Row>
                  {data.editing && (
                    <Section role={data.editorOverlay}>
                      <Card role="event-dialog">
                        <Row role="dialog-toolbar">
                          <Spacer />
                          {data.existing && (
                            <Button icon="trash-2" title="Delete event" onClick={() => onAction("delete-event")} />
                          )}
                          <Button icon="x" title="Close editor" onClick={() => onAction("close-editor")} />
                        </Row>
                        <Input role="event-title-input" field="title" placeholder="Add title" value={data.form.title} />
                        <Row role="event-types">
                          <Button onClick={() => onAction("type-event")} selected={data.eventType}>Event</Button>
                          <Button onClick={() => onAction("type-task")} selected={data.taskType}>Task</Button>
                          <Button onClick={() => onAction("type-appointment")} selected={data.appointmentType}>Appointment schedule</Button>
                        </Row>
                        {data.taskType && (
                          <Checkbox field="completed" label="Completed" checked={data.form.completed} />
                        )}
                        <Row role="form-row">
                          <Icon name="clock" />
                          <Input field="startDate" type="date" value={data.form.startDate} label="Start date" />
                          <Input field="startTime" type="time" value={data.form.startTime} label="Start time" disabled={data.form.allDay} />
                          <Text>–</Text>
                          <Input field="endTime" type="time" value={data.form.endTime} label="End time" disabled={data.form.allDay} />
                        </Row>
                        <Row role="advanced-indent">
                          <Checkbox field="allDay" label="All day" checked={data.form.allDay} />
                          <Input field="endDate" type="date" value={data.form.endDate} label="End date" />
                        </Row>
                        <Row role="form-indent">
                          <Select field="recurrence" label="Repeat" value={data.form.recurrence} options={data.repeatOptions} />
                          {data.repeating && (
                            <Select field="scope" label="Edit repeating event" value={data.form.scope} options={data.scopeOptions} />
                          )}
                        </Row>
                        <Row role="advanced-row">
                          <Icon name="globe" />
                          <Select field="timezone" label="Time zone" value={data.form.timezone} options={data.timezones} />
                        </Row>
                        <Row role="form-row">
                          <Icon name="users" />
                          <Input field="guests" placeholder="Add guests" value={data.form.guests} label="Guests" />
                        </Row>
                        <Row role="form-row">
                          <Icon name="video" />
                          <Input field="meetingUrl" placeholder="Add video conferencing" value={data.form.meetingUrl} label="Video conference URL" />
                        </Row>
                        <Row role="form-row">
                          <Icon name="map-pin" />
                          <Input field="location" placeholder="Add location" value={data.form.location} label="Location" />
                        </Row>
                        <Row role="advanced-row">
                          <Icon name="bell" />
                          <Select field="reminder" label="Notification" value={data.form.reminder} options={data.reminderOptions} />
                        </Row>
                        <Row role="form-row">
                          <Icon name="align-left" />
                          <Input field="description" multiline={true} placeholder="Add description or a Drive attachment" value={data.form.description} label="Description" />
                        </Row>
                        <Row role="form-row">
                          <Icon name="calendar-days" />
                          <Select field="calendar" label="Calendar" value={data.form.calendar} options={data.calendarOptions} disabled={data.existing} />
                          <Select field="availability" label="Show as" value={data.form.availability} options={data.availabilityOptions} />
                          <Select field="visibility" label="Visibility" value={data.form.visibility} options={data.visibilityOptions} />
                        </Row>
                        {data.error && (
                          <Alert tone="error" text={data.error} />
                        )}
                        <Row role="dialog-footer">
                          {data.repeating && (
                            <Text role="hint">Choose whether to update this occurrence or the entire series.</Text>
                          )}
                          <Spacer />
                          <Button role="more-options" onClick={() => onAction("more-options")}>More options</Button>
                          <Button role="editor-cancel" onClick={() => onAction("close-editor")}>Cancel</Button>
                          <Button variant="primary" onClick={() => onAction("save-event")} disabled={data.busy}>Save</Button>
                        </Row>
                      </Card>
                    </Section>
                  )}
                  {data.contactEditing && (
                    <Section role="modal-overlay">
                      <Card role="contact-dialog">
                        <Row>
                          <Heading level={2}>Contact</Heading>
                          <Spacer />
                          <Button icon="x" title="Close contact" onClick={() => onAction("close-contact")} />
                        </Row>
                        <Input field="name" label="Name" value={data.form.name} />
                        <Input field="email" label="Email" type="email" value={data.form.email} />
                        <Input field="phone" label="Phone" type="tel" value={data.form.phone} />
                        <Input field="company" label="Company" value={data.form.company} />
                        <Row>
                          {data.existing && (
                            <Button variant="danger" onClick={() => onAction("delete-contact")}>Delete</Button>
                          )}
                          <Spacer />
                          <Button variant="primary" onClick={() => onAction("save-contact")}>Save</Button>
                        </Row>
                      </Card>
                    </Section>
                  )}
                  {data.settingsOpen && (
                    <Section role="settings-overlay">
                      <Card role="settings-dialog">
                        <Row role="settings-header">
                          <Button icon="arrow-left" title="Back to calendar" onClick={() => onAction("close-settings")} />
                          <Heading level={1}>Settings</Heading>
                          <Spacer />
                          <Button variant="primary" onClick={() => onAction("save-settings")}>Save settings</Button>
                        </Row>
                        <Row role="settings-body">
                          <Stack role="settings-navigation">
                            <Text role="settings-nav-title">General</Text>
                            <Button onClick={() => onAction("settings-timezone")}>Time zone</Button>
                            <Button onClick={() => onAction("settings-events")}>Event settings</Button>
                            <Button onClick={() => onAction("settings-view")}>View options</Button>
                            <Button onClick={() => onAction("settings-import")}>Import & export</Button>
                            <Spacer />
                            <Link href={data.workspaceHome}>One Workspace</Link>
                          </Stack>
                          <Stack role="settings-content">
                            <Section role="timezone-settings">
                              <Heading level={2}>Time zone</Heading>
                              <Text role="hint">Your calendar uses this time zone to display event times.</Text>
                              <Input field="timezone" label="Primary time zone" value={data.form.timezone} />
                            </Section>
                            <Section role="event-settings">
                              <Heading level={2}>Event settings</Heading>
                              <Select field="eventDuration" label="Default event duration" value={data.form.eventDuration} options={data.durationOptions} />
                              <Select field="defaultReminder" label="Default notification" value={data.form.defaultReminder} options={data.reminderOptions} />
                              <Button onClick={() => onAction("notifications")}>Enable desktop notifications</Button>
                            </Section>
                            <Section role="view-settings">
                              <Heading level={2}>View options</Heading>
                              <Select field="weekStart" label="Start week on" value={data.form.weekStart} options={data.weekStartOptions} />
                              <Checkbox field="weekends" label="Show weekends" checked={data.form.weekends} />
                              <Checkbox field="dark" label="Dark appearance" checked={data.form.dark} />
                            </Section>
                            <Section role="import-settings">
                              <Heading level={2}>Import & export</Heading>
                              <Text role="hint">Import a standard iCalendar file from Google Calendar or another calendar app. Your events are saved in your shared Stalwart data folder.</Text>
                              <Row>
                                <Button onClick={() => onAction("import")}>Import .ics</Button>
                                <Button onClick={() => onAction("export")}>Export calendar</Button>
                              </Row>
                            </Section>
                            {data.error && (
                              <Alert tone="error" text={data.error} />
                            )}
                          </Stack>
                        </Row>
                      </Card>
                    </Section>
                  )}
                  {data.panel && (
                    <Section role="modal-overlay">
                      <Card role="info-dialog">
                        <Row>
                          <Heading level={2} text={data.panelTitle} />
                          <Spacer />
                          <Button icon="x" title="Close" onClick={() => onAction("close-panel")} />
                        </Row>
                        <Text role="panel-copy" text={data.panelText} />
                        {data.notesPanel && (
                          <>
                            <Input field="note" multiline={true} label="Your note" value={data.form.note} />
                            <Button variant="primary" onClick={() => onAction("save-notes")}>Save note</Button>
                          </>
                        )}
                        {data.accountPanel && (
                          <Button onClick={() => onAction("signout")}>Sign out</Button>
                        )}
                        {data.appLinks.map((item, i) => (
                          <Link key={i} href={item.href}>
                            {item.icon && (
                              <Image src={item.icon} alt="" width={24} />
                            )}
                            <Text text={item.title} />
                          </Link>
                        ))}
                      </Card>
                    </Section>
                  )}
                  {data.demo && (
                    <Text role="demo-badge">Demo</Text>
                  )}
                  {data.toast && (
                    <Alert role="toast" tone="success" text={data.toast} />
                  )}
                </>
              ) : (
                <Card role="login-card">
                  <Image src={data.appIcon} alt={data.appTitle} width={56} />
                  <Heading level={1} text={data.loginTitle} />
                  <Text text={data.loginDescription} />
                  <Link role="signin" href="auth/login">Sign in with MyAccounts</Link>
                  <Text role="hint" text={data.storageHint} />
                </Card>
              )}
            </Section>
          )}
        </Section>
      ) : (
        <Card role="login-card">
          <Heading level={1}>Opening your workspace</Heading>
          <Text>Loading your application…</Text>
        </Card>
      )}
    </Page>
  )
}
