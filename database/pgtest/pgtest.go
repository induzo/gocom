// Package pgtest implements dockertest with postgres for testing.
//
// Use case in TestMain
//
//	var dockertest *pgtest.DockertestWrapper
//
//	func TestMain(m *testing.M) {
//		dockertest = pgtest.New()
//
//		code := m.Run()
//
//		dockertest.Purge()
//
//		os.Exit(code)
//	}
//
//	func TestFunc(t *testing.T) {
//		t.Parallel()
//
//		repo := NewRepo(dockertest.ConnPool)
//
//		tests := []struct{
//			name string
//		}{}
//		for _, tt := range tests {
//			tt := tt
//			t.Run(tt.name, func(t *testing.T) {
//				t.Parallel()
//				...
//			})
//		}
//	}
//
//	func TestRun(t *testing.T) {
//		t.Parallel()
//
//		tests := []struct{
//			name string
//		}{}
//		for _, tt := range tests {
//			tt := tt
//			t.Run(tt.name, func(t *testing.T) {
//				t.Parallel()
//				repo := NewRepo(dockertest.ConnPool)
//				...
//			})
//		}
//	}
package pgtest
